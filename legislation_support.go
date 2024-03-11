package main

import (
	"context"
	"embed"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/dustin/go-humanize"
	"github.com/gomarkdown/markdown"
	"github.com/gorilla/handlers"
	"github.com/jehiah/legislation.support/internal/account"
	"github.com/jehiah/legislation.support/internal/legislature"
	"github.com/jehiah/legislation.support/internal/resolvers"
	"github.com/microcosm-cc/bluemonday"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

//go:embed templates/*
var content embed.FS

//go:embed static/*
var static embed.FS

var americaNewYork, _ = time.LoadLocation("America/New_York")

type App struct {
	devMode   bool
	firestore *firestore.Client
	firebase  *auth.Client

	staticHandler http.Handler
	templateFS    fs.FS
	firebaseAuth  http.Handler
}

func commaInt(i int) string {
	return humanize.Comma(int64(i))
}

func Markdown(md string) template.HTML {
	maybeUnsafeHTML := markdown.ToHTML([]byte(md), nil, nil)
	html := bluemonday.UGCPolicy().SanitizeBytes(maybeUnsafeHTML)
	return template.HTML(html)
}

func LegislationLink(b legislature.BodyID, l legislature.LegislationID) template.URL {
	return template.URL(resolvers.Resolvers.Find(b).Link(l).String())
}
func LegislationDisplayID(b legislature.BodyID, l legislature.LegislationID) string {
	return resolvers.Resolvers.Find(b).DisplayID(l)
}
func newTemplate(fs fs.FS, n string) *template.Template {
	funcMap := template.FuncMap{
		"ToLower":  strings.ToLower,
		"Comma":    commaInt,
		"Time":     humanize.Time,
		"Join":     strings.Join,
		"markdown": Markdown,
		// "Resolver": resolvers.Resolvers.Find,
		"LegislationLink":      LegislationLink,
		"LegislationDisplayID": LegislationDisplayID,
	}
	t := template.New("empty").Funcs(funcMap)
	if n == "error.html" {
		return template.Must(t.ParseFS(fs, filepath.Join("templates", n)))
	}
	return template.Must(t.ParseFS(fs, filepath.Join("templates", n), "templates/base.html"))
}

// RobotsTXT renders /robots.txt
func (a *App) RobotsTXT(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("content-type", "text/plain")
	a.addExpireHeaders(w, time.Hour*24*7)
	io.WriteString(w, "# robots welcome\n# https://github.com/jehiah/legislation.support\n")
}

type LastSync struct {
	LastRun time.Time
}

func (a *App) addExpireHeaders(w http.ResponseWriter, duration time.Duration) {
	if a.devMode {
		return
	}
	w.Header().Add("Cache-Control", fmt.Sprintf("public; max-age=%d", int(duration.Seconds())))
	w.Header().Add("Expires", time.Now().Add(duration).Format(http.TimeFormat))
}

type BillBody struct {
	Legislation legislature.Legislation
	Body        legislature.Body
}

func (a *App) SUSI(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	type Page struct {
		Page       string
		Title      string
		UID        account.UID
		Bills      []BillBody
		AuthDomain string
	}

	var bills []legislature.Legislation
	var err error

	if r.URL.Path == "/" {
		bills, err = a.GetRecentBills(ctx, 10)
		if err != nil {
			log.Print(err)
			a.WebInternalError500(w, "")
			return
		}
	}

	body := Page{
		Title:      "legislation.support",
		AuthDomain: "legislation.support",
	}
	if a.devMode {
		body.AuthDomain = "dev.legislation.support"
	}

	for _, b := range bills {
		body.Bills = append(body.Bills, BillBody{
			Legislation: b,
			Body:        resolvers.Bodies[b.Body],
		})
	}

	t := newTemplate(a.templateFS, "susi.html")
	err = t.ExecuteTemplate(w, "susi.html", body)
	if err != nil {
		log.Print(err)
		a.WebInternalError500(w, "")
	}
	return
}

func (a *App) WebInternalError500(w http.ResponseWriter, msg string) {
	if msg == "" {
		msg = "Server Error"
	}
	a.WebError(w, 500, msg)
}
func (a *App) WebPermissionError403(w http.ResponseWriter, msg string) {
	if msg == "" {
		msg = "Permission Denied"
	}
	a.WebError(w, 403, msg)
}

func (a *App) WebError(w http.ResponseWriter, code int, msg string) {
	type Page struct {
		Title   string
		Code    int
		Message string
	}
	t := newTemplate(a.templateFS, "error.html")
	err := t.ExecuteTemplate(w, "error.html", Page{
		Title:   fmt.Sprintf("HTTP Error %d", code),
		Code:    code,
		Message: msg,
	})
	if err != nil {
		log.Errorf("%s", err)
	}
}

type ProfileMetadata struct {
	account.Profile

	SupportedBills int
	OpposedBills   int
	ArchivedBills  int
}

func (a *App) Index(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := a.User(r)
	if uid == "" {
		a.SUSI(w, r)
		return
	}

	type Page struct {
		Page     string
		Title    string
		UID      account.UID
		Profiles []ProfileMetadata
	}
	body := Page{
		Title: "legislation.support",
		UID:   uid,
	}

	profiles, err := a.GetProfiles(ctx, uid)
	if err != nil {
		log.Print(err)
		a.WebInternalError500(w, "")
		return
	}

	for _, p := range profiles {
		profile := ProfileMetadata{
			Profile: p,
		}
		b, err := a.GetProfileBookmarks(ctx, p.ID)
		if err != nil {
			log.WithField("uid", uid).WithField("profileID", p.ID).Errorf("%#v", err)
			a.WebInternalError500(w, "")
			return
		}
		for _, bb := range b {
			if bb.LastModified.After(profile.LastModified) {
				profile.LastModified = bb.LastModified
			}
			if bb.Legislation.Session.Active() {
				if bb.Oppose {
					profile.OpposedBills++
				} else {
					profile.SupportedBills++
				}
			} else {
				profile.ArchivedBills++
			}
		}
		body.Profiles = append(body.Profiles, profile)
	}

	sort.Slice(body.Profiles, func(i, j int) bool {
		return body.Profiles[i].LastModified.After(body.Profiles[j].LastModified)
	})

	t := newTemplate(a.templateFS, "profiles.html")
	err = t.ExecuteTemplate(w, "profiles.html", body)
	if err != nil {
		log.Print(err)
		a.WebInternalError500(w, "")
	}
	return
}

func (a *App) IndexPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uid := a.User(r)
	if uid == "" {
		http.Redirect(w, r, "/", 302)
		return
	}
	r.ParseForm()

	profile := account.Profile{
		Name:    strings.TrimSpace(r.PostForm.Get("name")),
		ID:      account.ProfileID(r.PostForm.Get("id")),
		UID:     uid,
		Created: time.Now().UTC(),
	}

	if !account.IsValidProfileID(profile.ID) {
		log.WithField("uid", uid).Infof("profile ID %q is invalid", profile.ID)
		http.Error(w, fmt.Sprintf("profile ID %q is invalid", profile.ID), 422)
		return
	}

	if profile.Name == "" {
		profile.Name = string(profile.ID)
	}

	err := a.CreateProfile(ctx, profile)
	if err != nil {
		// duplicate?
		log.WithField("uid", uid).Warningf("%#v %s", err, err)
		http.Error(w, fmt.Sprintf("profile %q is already taken", profile.ID), 409)
		return
	}
	http.Redirect(w, r, profile.Link(), 302)
}

func (a *App) Profile(w http.ResponseWriter, r *http.Request, profileID account.ProfileID) {
	ctx := r.Context()
	uid := a.User(r)

	profile, err := a.GetProfile(ctx, profileID)
	if err != nil {
		log.WithField("uid", uid).WithField("profileID", profileID).Errorf("%#v", err)
		a.WebInternalError500(w, "")
		return
	}
	if profile == nil {
		http.Error(w, "Not Found", 404)
		return
	}

	if uid == "" && profile.Private {
		a.WebPermissionError403(w, "")
		return
	}
	a.ShowProfile(w, ctx, uid, profile, Message{})
}
func (a *App) ShowProfile(w http.ResponseWriter, ctx context.Context, uid account.UID, profile *account.Profile, message Message) {
	templateName := "profile.html"
	t := newTemplate(a.templateFS, "profile.html")
	profileID := profile.ID

	type Page struct {
		Page              string
		Title             string
		Message           Message
		UID               account.UID
		Profile           account.Profile
		EditMode          bool
		Bookmarks         account.Bookmarks
		ArchivedBookmarks account.Bookmarks
	}
	body := Page{
		Message:  message,
		Title:    profile.Name + " (legislation.support)",
		Profile:  *profile,
		EditMode: uid == profile.UID,
		UID:      uid,
	}
	if body.EditMode {
		templateName = "profile_edit.html"
		t = newTemplate(a.templateFS, "profile_edit.html")
	}
	b, err := a.GetProfileBookmarks(ctx, profileID)
	if err != nil {
		log.WithField("uid", uid).WithField("profileID", profileID).Errorf("%#v", err)
		a.WebInternalError500(w, "")
		return
	}
	for _, bb := range b {
		if bb.Legislation.Session.Active() {
			body.Bookmarks = append(body.Bookmarks, bb)
		} else {
			body.ArchivedBookmarks = append(body.ArchivedBookmarks, bb)
		}
	}

	sort.Sort(account.SortedBookmarks(body.Bookmarks))
	sort.Sort(account.SortedBookmarks(body.ArchivedBookmarks))
	// log.Printf("bookmarks %#v", body.Bookmarks)

	err = t.ExecuteTemplate(w, templateName, body)
	if err != nil {
		log.WithField("uid", uid).Error(err)
		a.WebInternalError500(w, "")
	}
}

type Message struct {
	Success string
	Error   string
}

// ProfilePost handles the add of a new URL to a profile, or update of a profile
func (a *App) ProfilePost(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	ctx := r.Context()
	uid := a.User(r)

	profileID := account.ProfileID(r.Form.Get("profile_id"))
	logFields := log.Fields{"uid": uid, "profileID": profileID}

	profile, err := a.GetProfile(ctx, profileID)
	if err != nil {
		log.WithContext(ctx).WithFields(logFields).Errorf("%#v", err)
		a.WebInternalError500(w, "")
		return
	}
	if profile == nil {
		http.Error(w, "Not Found", 404)
		return
	}

	if !profile.HasAccess(uid) {
		a.WebPermissionError403(w, "")
		return
	}

	switch {
	case strings.TrimSpace(r.Form.Get("legislation_url")) != "":
		var msg Message
		added, err := a.ProfilePostURL(ctx, profileID, r)
		if added > 0 {
			msg.Success = fmt.Sprintf("Added %d", added)
		}
		if err != nil {
			msg.Error = err.Error()
		}
		a.ShowProfile(w, ctx, uid, profile, msg)
		return
	case strings.TrimSpace(r.Form.Get("name")) != "":
		err = a.ProfileEdit(ctx, *profile, r)
	}

	if err != nil {
		log.WithContext(ctx).WithFields(logFields).Errorf("%#v", err)
		a.WebInternalError500(w, "")
		return
	}
	http.Redirect(w, r, profile.Link(), 302)
}

func (a *App) ProfileEdit(ctx context.Context, p account.Profile, r *http.Request) error {
	p.Name = strings.TrimSpace(r.Form.Get("name"))
	if p.Name == "" {
		return fmt.Errorf("name required")
	}
	p.Description = strings.TrimSpace(r.Form.Get("description"))
	p.Private = r.Form.Get("private") == "on"
	p.HideDistrict = r.Form.Get("hide_district") == "on"
	p.HideSupportOppose = r.Form.Get("hide_support_oppose") == "on"
	p.HideBillStatus = r.Form.Get("hide_bill_status") == "on"
	return a.UpdateProfile(ctx, p)
}

func (a *App) ProfilePostURL(ctx context.Context, profileID account.ProfileID, r *http.Request) (int64, error) {
	uid := a.User(r)
	g := new(errgroup.Group)
	// support multiple URL's
	var added int64

	for _, legUrl := range strings.Fields(strings.TrimSpace(r.Form.Get("legislation_url"))) {
		legUrl := legUrl

		g.Go(func() error {

			u, err := url.Parse(legUrl)
			if err != nil {
				return err
			}
			logFields := log.Fields{"uid": uid, "profileID": profileID, "legislation_url": u.String()}
			log.WithContext(ctx).WithFields(logFields).Infof("parsed URL")
			bill, err := resolvers.Resolvers.Lookup(r.Context(), u)
			if err != nil {
				return err
			}
			if bill == nil {
				return fmt.Errorf("Legislation matching url %q not found", u)
			}

			// Save refreshes a bill as well
			err = a.SaveBill(ctx, *bill)
			if err != nil {
				return err
			}

			bookmark, err := a.GetBookmark(ctx, profileID, account.BookmarkKey(bill.Body, bill.ID))
			if err != nil {
				return err
			}
			if bookmark != nil {
				bookmark.Notes = strings.TrimSpace(r.Form.Get("notes"))
				bookmark.Tags = strings.Fields(strings.TrimSpace(r.Form.Get("tags")))
				bookmark.Oppose = r.Form.Get("support") == "👎"

				// update
				err = a.UpdateBookmark(ctx, profileID, *bookmark)
				if err != nil {
					return err
				}
				atomic.AddInt64(&added, 1)
				return nil
			}

			oppose := r.Form.Get("support") == "👎"
			err = a.SaveBookmark(ctx, profileID, account.Bookmark{
				UID:           uid,
				BodyID:        bill.Body,
				LegislationID: bill.ID,
				Oppose:        oppose,
				Created:       time.Now().UTC(),
				Notes:         strings.TrimSpace(r.Form.Get("notes")),
				Tags:          strings.Fields(strings.TrimSpace(r.Form.Get("tags"))),
			})
			if err != nil && IsAlreadyExists(err) {
				return nil
			} else if err == nil {
				// results
				atomic.AddInt64(&added, 1)
			}
			return nil
		})
	}
	return added, g.Wait()

}

// ProfileRemove removes a bookmark from a profile
func (a *App) ProfileRemove(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(2 << 20) // 2Mb
	if err != nil {
		log.Printf("err parsing form %s", err)
	}
	ctx := r.Context()
	uid := a.User(r)
	profileID := account.ProfileID(r.PostForm.Get("profile_id"))
	body, legID := legislature.BodyID(r.PostForm.Get("body_id")), legislature.LegislationID(r.PostForm.Get("legislation_id"))
	logFields := log.Fields{"uid": uid, "profileID": profileID, "body": body, "legislation_id": legID}

	profile, err := a.GetProfile(ctx, profileID)
	if err != nil {
		log.WithContext(ctx).WithFields(logFields).Errorf("%#v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	if profile == nil {
		http.Error(w, "Not Found", 404)
		return
	}

	if !profile.HasAccess(uid) {
		http.Error(w, "Permission Denied.", 403)
		return
	}

	err = a.DeleteBookmark(ctx, profileID, body, legID)
	if err != nil {
		log.WithContext(ctx).WithFields(logFields).Errorf("%#v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, profile.Link(), 302)
	return
}

func (app App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// https://firebase.google.com/docs/auth/web/redirect-best-practices#proxy-requests
	if strings.HasPrefix(r.URL.Path, "/__/auth") {
		// reverse proxy for signin-helpers for popup/redirect sign in
		// for Safari/iOS
		app.firebaseAuth.ServeHTTP(w, r)
		return
	}
	switch r.Method {
	case "GET":
		switch r.URL.Path {
		case "/":
			app.Index(w, r)
			return
		case "/sign_in":
			app.SUSI(w, r)
			return
		case "/sign_out":
			app.SignOut(w, r)
			return
		case "/robots.txt":
			app.RobotsTXT(w, r)
			return
		}
		if strings.HasPrefix(r.URL.Path, "/static/") {
			app.staticHandler.ServeHTTP(w, r)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/scorecard") || strings.Contains(r.URL.Path, "/scorecard/") {
			s := strings.Split(r.URL.Path, "/")
			// /$profile/scorecard or /$profile/scorecard/$bodyID
			if p := account.ProfileID(s[1]); account.IsValidProfileID(p) {
				var b legislature.BodyID
				if len(s) == 3 {
					app.Scorecard(w, r, p, b)
					return
				} else if len(s) == 4 && resolvers.IsValidBodyID(legislature.BodyID(s[3])) {
					b = legislature.BodyID(s[3])
					if resolvers.IsValidBodyID(b) {
						app.Scorecard(w, r, p, b)
						return
					}
				}
			}
		}

		if p := account.ProfileID(strings.TrimPrefix(r.URL.Path, "/")); account.IsValidProfileID(p) {
			app.Profile(w, r, p)
			return
		}
	case "POST":
		switch r.URL.Path {
		case "/":
			app.IndexPost(w, r)
			return
		case "/data/profile":
			app.ProfilePost(w, r)
			return
		case "/data/session":
			app.NewSession(w, r)
			return
		}
	case "DELETE":
		switch r.URL.Path {
		case "/data/profile":
			app.ProfileRemove(w, r)
			return
		}
	default:
		http.Error(w, "Invalid Method", http.StatusMethodNotAllowed)
		return
	}
	http.NotFound(w, r)
}

// tsFmt is used to match logrus timestamp format
// w/ our stdlib log fmt (Ldate | Ltime)
const tsFmt = "2006/01/02 15:04:05"

func main() {
	logRequests := flag.Bool("log-requests", false, "log requests")
	devMode := flag.Bool("dev-mode", false, "development mode")
	flag.Parse()
	log.SetReportCaller(true)
	if *devMode {
		*logRequests = true
		log.SetFormatter(&log.TextFormatter{TimestampFormat: tsFmt, FullTimestamp: true})
	} else {
		log.SetFormatter(&fluentdFormatter{})
	}

	log.Print("starting server...")
	ctx := context.Background()
	firebaseApp, err := firebase.NewApp(ctx, &firebase.Config{
		ProjectID:        "legislation-support",
		ServiceAccountID: "firebase-adminsdk-q48s8@legislation-support.iam.gserviceaccount.com",
	})
	if err != nil {
		log.Fatal(err)
	}
	authClient, err := firebaseApp.Auth(ctx)
	if err != nil {
		log.Fatal(err)
	}
	firebase := &url.URL{Scheme: "https", Host: "legislation-support.firebaseapp.com"}
	app := &App{
		devMode:       *devMode,
		firestore:     createClient(ctx),
		firebase:      authClient,
		staticHandler: http.FileServer(http.FS(static)),
		templateFS:    content,
		firebaseAuth: &httputil.ReverseProxy{
			Rewrite: func(r *httputil.ProxyRequest) {
				r.SetXForwarded()
				r.SetURL(firebase)
			},
		},
	}
	if *devMode {
		app.templateFS = os.DirFS(".")
		app.staticHandler = http.StripPrefix("/static/", http.FileServer(http.Dir("static")))
	}

	// Determine port for HTTP service.
	port := os.Getenv("PORT")
	if port == "" {
		if *devMode {
			port = "443"
		} else {
			port = "8081"
		}
	}

	var h http.Handler = app
	if *logRequests {
		h = handlers.LoggingHandler(os.Stdout, h)
	}

	// Start HTTP server.

	if *devMode {
		// mkcert -key-file dev/key.pem -cert-file dev/cert.pem dev.legislation.support
		if _, err := os.Stat("dev/cert.pem"); os.IsNotExist(err) {
			log.Printf("dev/cert.pem missing.")
			os.Mkdir("dev", 0750)
			cmd := exec.Command("mkcert", "-install", "-key-file=dev/key.pem", "-cert-file=dev/cert.pem", "dev.legislation.support")
			cmd.Stdout = os.Stdout
			cmd.Stderr = os.Stderr
			log.Printf("%s %s", cmd.Path, strings.Join(cmd.Args[1:], " "))
			err := cmd.Run()
			if err != nil {
				log.Fatal(err)
			}
		}
		log.Printf("listening to HTTPS on port %s https://dev.legislation.support", port)
		if err := http.ListenAndServeTLS(":"+port, "dev/cert.pem", "dev/key.pem", h); err != nil {
			log.Fatal(err)
		}
	} else {
		log.Printf("listening on port %s", port)
		if err := http.ListenAndServe(":"+port, h); err != nil {
			log.Fatal(err)
		}
	}

}
