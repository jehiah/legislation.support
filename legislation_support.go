package main

import (
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

	"github.com/dustin/go-humanize"
	"github.com/gorilla/handlers"
	"github.com/jehiah/legislation.support/internal/resolvers"
	"github.com/julienschmidt/httprouter"
)

//go:embed templates/*
var content embed.FS

//go:embed static/*
var static embed.FS

var americaNewYork, _ = time.LoadLocation("America/New_York")

type App struct {
	devMode bool

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

func (a *App) Index(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	t := newTemplate(a.templateFS, "index.html")
	type Page struct {
		Page  string
		Title string
	}
	body := Page{
		Title: "legislation.support",
	}
	err := t.ExecuteTemplate(w, "index.html", body)
	if err != nil {
		log.Print(err)
		http.Error(w, "Internal Server Error", 500)
	}
	return
}

func (a *App) IndexPost(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	r.ParseForm()
	u, err := url.Parse(r.Form.Get("legislation_url"))
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	log.Printf("%s", u.String())
	body, err := resolvers.Resolvers.Lookup(r.Context(), u)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	json.NewEncoder(w).Encode(body)
}

func main() {
	logRequests := flag.Bool("log-requests", false, "log requests")
	devMode := flag.Bool("dev-mode", false, "development mode")
	flag.Parse()

	log.Print("starting server...")

	app := &App{
		devMode:       *devMode,
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
