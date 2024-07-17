package main

import (
	"context"
	"crypto/tls"
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
	"strings"
	"time"

	firebase "firebase.google.com/go/v4"
	"firebase.google.com/go/v4/auth"
	"github.com/dustin/go-humanize"
	"github.com/gomarkdown/markdown"
	"github.com/gorilla/handlers"
	"github.com/jehiah/legislation.support/internal/account"
	"github.com/jehiah/legislation.support/internal/datastore"
	"github.com/jehiah/legislation.support/internal/legislature"
	"github.com/jehiah/legislation.support/internal/resolvers"
	"github.com/microcosm-cc/bluemonday"
	log "github.com/sirupsen/logrus"
)

//go:embed templates/*
var content embed.FS

//go:embed static/*
var static embed.FS

// var americaNewYork, _ = time.LoadLocation("America/New_York")

type App struct {
	devMode  bool
	firebase *auth.Client

	staticHandler http.Handler
	templateFS    fs.FS
	firebaseAuth  http.Handler

	*datastore.Datastore
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
func LookupBody(b legislature.BodyID) legislature.Body {
	return resolvers.Bodies[b]
}
func JoinTags(tags []account.DisplayTag) string {
	var out []string
	for _, t := range tags {
		out = append(out, t.Tag)
	}
	return strings.Join(out, "|")

}

func newTemplate(fs fs.FS, n string) *template.Template {
	funcMap := template.FuncMap{
		"ToLower":  strings.ToLower,
		"Comma":    commaInt,
		"Time":     humanize.Time,
		"Join":     strings.Join,
		"JoinTags": JoinTags,
		"markdown": Markdown,
		// "Resolver": resolvers.Resolvers.Find,
		"LegislationLink":      LegislationLink,
		"LegislationDisplayID": LegislationDisplayID,
		"LookupBody":           LookupBody,
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
	io.WriteString(w, `# robots welcome
# https://github.com/jehiah/legislation.support

User-agent: *
Disallow: /__/
`)
}

func (a *App) addExpireHeaders(w http.ResponseWriter, duration time.Duration) {
	if a.devMode {
		return
	}
	w.Header().Add("Cache-Control", fmt.Sprintf("public; max-age=%d", int(duration.Seconds())))
	w.Header().Add("Expires", time.Now().Add(duration).Format(http.TimeFormat))
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
	w.WriteHeader(code)
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

	// nyassembly.gov SSL has invalid chain
	// https://www.ssllabs.com/ssltest/analyze.html?d=nyassembly.gov
	t := http.DefaultTransport.(*http.Transport)
	t.TLSClientConfig = &tls.Config{
		InsecureSkipVerify: true,
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
		Datastore:     datastore.New(datastore.NewClient(ctx)),
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

	router := http.NewServeMux()
	router.HandleFunc("GET /{$}", app.Index)
	router.HandleFunc("POST /{$}", app.IndexPost)
	router.HandleFunc("GET /sign_in", app.SUSI)
	router.HandleFunc("GET /sign_out", app.SignOut)
	router.HandleFunc("GET /robots.txt", app.RobotsTXT)
	if app.devMode {
		router.HandleFunc("GET /internal/refresh", app.InternalRefresh)
	}
	router.HandleFunc("GET /{profile}", app.Profile)
	router.HandleFunc("GET /{profile}/changes", app.ProfileChanges)
	router.HandleFunc("GET /{profile}/changes.xml", app.ProfileChanges)  // RSS
	router.HandleFunc("GET /{profile}/changes.json", app.ProfileChanges) // Json feed
	router.HandleFunc("GET /{profile}/scorecard/{body}", app.Scorecard)

	router.HandleFunc("POST /data/profile", app.ProfilePost)
	router.HandleFunc("DELETE /data/profile", app.ProfileRemove)
	router.HandleFunc("POST /data/session", app.NewSession)
	router.HandleFunc("POST /internal/refresh", app.InternalRefresh)

	wrapper := http.NewServeMux()
	wrapper.Handle("/", router)
	wrapper.HandleFunc("GET /static/", app.staticHandler.ServeHTTP)
	// https://firebase.google.com/docs/auth/web/redirect-best-practices#proxy-requests
	// reverse proxy for signin-helpers for popup/redirect sign in
	// for Safari/iOS
	wrapper.Handle("/__/auth/", app.firebaseAuth)

	// Determine port for HTTP service.
	port := os.Getenv("PORT")
	if port == "" {
		if *devMode {
			port = "443"
		} else {
			port = "8081"
		}
	}

	var h http.Handler = wrapper
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
