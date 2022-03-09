package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/fs"
	"log"
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
	"github.com/jehiah/legislation.support/internal/legislature"
	"github.com/jehiah/legislation.support/internal/resolvers"
	"github.com/julienschmidt/httprouter"
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
func (a *App) RobotsTXT(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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

func (a *App) SUSI(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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

func (a *App) Index(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	uid := a.User(r)
	if uid == "" {
		a.SUSI(w, r, ps)
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

func (a *App) IndexPost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	ctx := r.Context()
	uid := a.User(r)
	if uid == "" {
		http.Redirect(w, r, "/", 302)
		return
	}
	r.ParseForm()

	profile := account.Profile{
		Name:    r.PostForm.Get("name"),
		ID:      account.ProfileID(r.PostForm.Get("id")),
		UID:     uid,
		Created: time.Now().UTC(),
	}

	if !account.IsValidProfileID(profile.ID) {
		http.Error(w, fmt.Sprintf("profile %q is invalid", profile.ID), 422)
		return
	}

	err := a.CreateProfile(ctx, profile)
	if err != nil {
		// duplicate?
		log.Printf("%#v %s", err, err)
		http.Error(w, fmt.Sprintf("profile %q is already taken", profile.ID), 409)
		return
	}
	http.Redirect(w, r, "/profile/"+url.PathEscape(string(profile.ID)), 302)
}

func (a *App) ProfilePost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	r.ParseForm()
	ctx := r.Context()
	uid := a.User(r)
	if uid == "" {
		http.Redirect(w, r, "/", 302)
		return
	}

	profileID := account.ProfileID(ps.ByName("profile"))
	if !account.IsValidProfileID(profileID) {
		http.Error(w, fmt.Sprintf("profile %q is invalid", profileID), 422)
		return
	}

	profile, err := a.GetProfile(ctx, profileID)
	if err != nil {
		log.Print(err)
		http.Error(w, "Internal Server Error", 500)
		return
	}

	if profile == nil || profile.UID != uid {
		http.Error(w, "Permission Denied.", 403)
		return
	}

	u, err := url.Parse(r.Form.Get("legislation_url"))
	if err != nil {
		http.Error(w, err.Error(), 422)
		return
	}
	log.Printf("%s", u.String())
	bill, err := resolvers.Resolvers.Lookup(r.Context(), u)
	if err != nil {
		log.Printf("%s", err)
		http.Error(w, err.Error(), 500)
		return
	}
	if bill != nil {
		err = a.SaveBill(ctx, *bill)
		if err != nil {
			log.Printf("%s", err)
			http.Error(w, err.Error(), 500)
			return
		}
		oppose := r.Form.Get("support") == "👎"
		err = a.SaveBookmark(ctx, profileID, account.Bookmark{
			UID:           uid,
			Body:          bill.Body,
			LegislationID: bill.ID,
			Oppose:        oppose,
			Created:       time.Now().UTC(),
		})
		if err != nil {
			log.Printf("%s", err)
			http.Error(w, err.Error(), 500)
			return
		}
	}
	json.NewEncoder(w).Encode(bill)
}

func (a *App) Profile(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t := newTemplate(a.templateFS, "profile.html")
	profileID := account.ProfileID(ps.ByName("profile"))
	ctx := r.Context()
	uid := a.User(r)

	profile, err := a.GetProfile(ctx, profileID)
	if err != nil {
		log.Printf("%#v", err)
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
		Page    string
		Title   string
		UID     account.UID
		Profile account.Profile
		Bills   []legislature.Legislation
	}
	body := Page{
		Title:   profile.Name + " (legislation.support)",
		Profile: *profile,
		UID:     uid,
	}
	body.Bills, err = a.GetProfileBills(ctx, profileID)
	if err != nil {
		log.Printf("%#v", err)
		http.Error(w, err.Error(), 500)
		return
	}

	err = t.ExecuteTemplate(w, "profile.html", body)
	if err != nil {
		log.Print(err)
		http.Error(w, "Internal Server Error", 500)
	}
}

func main() {
	logRequests := flag.Bool("log-requests", false, "log requests")
	devMode := flag.Bool("dev-mode", false, "development mode")
	flag.Parse()

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

	router := httprouter.New()
	router.GET("/", app.Index)
	router.POST("/", app.IndexPost)
	router.POST("/session", app.NewSession)
	router.GET("/sign_out", app.SignOut)
	router.GET("/profile/:profile", app.Profile)
	router.GET("/robots.txt", app.RobotsTXT)
	router.Handler("GET", "/static/*file", app.staticHandler)

	// Determine port for HTTP service.
	port := os.Getenv("PORT")
	if port == "" {
		port = "8081"
	}

	var h http.Handler = router
	if *logRequests {
		h = handlers.LoggingHandler(os.Stdout, h)
	}

	// Start HTTP server.
	log.Printf("listening on port %s", port)
	if err := http.ListenAndServe(":"+port, h); err != nil {
		log.Fatal(err)
	}
}
