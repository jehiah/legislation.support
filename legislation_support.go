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
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cloud.google.com/go/firestore"
	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/dustin/go-humanize"
	"github.com/gorilla/handlers"
	"github.com/jehiah/legislation.support/internal/account"
	"github.com/jehiah/legislation.support/internal/resolvers"
	log "github.com/sirupsen/logrus"
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
}

func commaInt(i int) string {
	return humanize.Comma(int64(i))
}

func newTemplate(fs fs.FS, n string) *template.Template {
	funcMap := template.FuncMap{
		"ToLower": strings.ToLower,
		"Comma":   commaInt,
		"Time":    humanize.Time,
	}
	t := template.New("empty").Funcs(funcMap)
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

func (a *App) SUSI(w http.ResponseWriter, r *http.Request) {
	type Page struct {
		Page  string
		Title string
		UID   account.UID
	}
	body := Page{
		Title: "legislation.support",
	}
	t := newTemplate(a.templateFS, "susi.html")
	err := t.ExecuteTemplate(w, "susi.html", body)
	if err != nil {
		log.Print(err)
		http.Error(w, "Internal Server Error", 500)
	}
	return
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
		Profiles []account.Profile
	}
	body := Page{
		Title: "legislation.support",
		UID:   uid,
	}
	var err error
	body.Profiles, err = a.GetProfiles(ctx, uid)
	if err != nil {
		log.Print(err)
		http.Error(w, "Internal Server Error", 500)
		return
	}

	t := newTemplate(a.templateFS, "profiles.html")
	err = t.ExecuteTemplate(w, "profiles.html", body)
	if err != nil {
		log.Print(err)
		http.Error(w, "Internal Server Error", 500)
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
	t := newTemplate(a.templateFS, "profile.html")
	ctx := r.Context()
	uid := a.User(r)

	profile, err := a.GetProfile(ctx, profileID)
	if err != nil {
		log.WithField("uid", uid).WithField("profileID", profileID).Errorf("%#v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	if profile == nil {
		http.Error(w, "Not Found", 404)
		return
	}

	if uid == "" && profile.Private {
		http.Error(w, "Permission Denied.", 403)
		return
	}

	type Page struct {
		Page      string
		Title     string
		UID       account.UID
		Profile   account.Profile
		EditMode  bool
		Bookmarks []account.Bookmark
	}
	body := Page{
		Title:    profile.Name + " (legislation.support)",
		Profile:  *profile,
		EditMode: uid == profile.UID,
		UID:      uid,
	}
	body.Bookmarks, err = a.GetProfileBookmarks(ctx, profileID)
	if err != nil {
		log.WithField("uid", uid).WithField("profileID", profileID).Errorf("%#v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	// log.Printf("bookmarks %#v", body.Bookmarks)

	err = t.ExecuteTemplate(w, "profile.html", body)
	if err != nil {
		log.WithField("uid", uid).Error(err)
		http.Error(w, "Internal Server Error", 500)
	}
}

func (a *App) ProfilePost(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()
	ctx := r.Context()
	uid := a.User(r)

	profileID := account.ProfileID(r.Form.Get("profile_id"))

	profile, err := a.GetProfile(ctx, profileID)
	if err != nil {
		log.WithField("uid", uid).WithField("profileID", profileID).Errorf("%#v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	if profile == nil {
		http.Error(w, "Not Found", 404)
		return
	}

	if uid != profile.UID {
		http.Error(w, "Permission Denied.", 403)
		return
	}
	legUrl := strings.TrimSpace(r.Form.Get("legislation_url"))
	u, err := url.Parse(legUrl)
	if err != nil {
		log.WithField("uid", uid).WithField("profileID", profileID).Warningf("%s", err)
		http.Error(w, err.Error(), 422)
		return
	}
	log.WithField("uid", uid).WithField("profileID", profileID).Infof("%s", u.String())
	bill, err := resolvers.Resolvers.Lookup(r.Context(), u)
	if err != nil {
		log.WithField("uid", uid).WithField("profileID", profileID).WithField("legislation_url", legUrl).Errorf("%s", err)
		http.Error(w, err.Error(), 500)
		return
	}
	if bill == nil {
		log.WithField("uid", uid).WithField("profileID", profileID).WithField("legislation_url", legUrl).Info("matching legislation not found")
		http.Error(w, fmt.Sprintf("Legislation matching url %q not found", u.String()), 422)
		return
	}

	err = a.SaveBill(ctx, *bill)
	if err != nil {
		if IsAlreadyExists(err) {
			a.UpdateBill(ctx, *bill)
		} else {
			log.WithField("uid", uid).WithField("profileID", profileID).Errorf("%#v", err)
			http.Error(w, err.Error(), 500)
			return
		}
	}

	bookmark, err := a.GetBookmark(ctx, profileID, account.BookmarkKey(*bill))
	if err != nil {
		log.WithField("uid", uid).WithField("profileID", profileID).Errorf("%#v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	if bookmark != nil {
		bookmark.Notes = strings.TrimSpace(r.Form.Get("notes"))
		bookmark.Tags = strings.Fields(strings.TrimSpace(r.Form.Get("tags")))
		bookmark.Oppose = r.Form.Get("support") == "👎"

		// update
		err = a.UpdateBookmark(ctx, profileID, *bookmark)
		if err != nil {
			log.WithField("uid", uid).WithField("profileID", profileID).Errorf("%#v", err)
			http.Error(w, err.Error(), 500)
			return
		}
		http.Redirect(w, r, profile.Link(), 302)
		return
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
	if err != nil && !IsAlreadyExists(err) {
		log.WithField("uid", uid).WithField("profileID", profileID).Errorf("%#v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, profile.Link(), 302)
}

func (app App) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		switch r.URL.Path {
		case "/":
			app.Index(w, r)
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

	app := &App{
		devMode:       *devMode,
		firestore:     createClient(ctx),
		firebase:      authClient,
		staticHandler: http.FileServer(http.FS(static)),
		templateFS:    content,
	}
	if *devMode {
		app.templateFS = os.DirFS(".")
		app.staticHandler = http.StripPrefix("/static/", http.FileServer(http.Dir("static")))
	}

	// Determine port for HTTP service.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	var h http.Handler = app
	if *logRequests {
		h = handlers.LoggingHandler(os.Stdout, h)
	}

	// Start HTTP server.
	log.Printf("listening on port %s", port)
	if err := http.ListenAndServe(":"+port, h); err != nil {
		log.Fatal(err)
	}
}
