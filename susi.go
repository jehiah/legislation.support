package main

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/jehiah/legislation.support/internal/account"
	"github.com/jehiah/legislation.support/internal/legislature"
	"github.com/jehiah/legislation.support/internal/resolvers"
	log "github.com/sirupsen/logrus"
)

type SessionRequest struct {
	IDToken string `json:"id_token"`
}

func (a *App) User(r *http.Request) account.UID {
	cookie, err := r.Cookie("session")
	if err != nil {
		return ""
	}
	// VerifySessionCookieAndCheckRevoked would make a server side call to check if it's revoked
	decoded, err := a.firebase.VerifySessionCookie(r.Context(), cookie.Value)
	if err != nil {
		return ""
	}
	return account.UID(decoded.UID)
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

	body := Page{
		Title:      "What Legislation Do You Support?",
		AuthDomain: "legislation.support",
	}

	if r.URL.Path == "/" {
		bills, err = a.GetRecentBills(ctx, 10)
		if err != nil {
			log.Print(err)
			a.WebInternalError500(w, "")
			return
		}
	} else {
		body.Title = "Sign In"
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

func (a *App) SignOut(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		MaxAge:   0,
		HttpOnly: true,
		Secure:   !a.devMode,
		Path:     "/",
	})
	http.Redirect(w, r, "/", 302)
}

// NewSession handles POST /data/session at the end of authentication
func (a *App) NewSession(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var body SessionRequest
	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		log.Printf("%#v", err)
		http.Error(w, "invalid json", 422)
		return
	}

	expiresIn := time.Hour * 24 * 13

	// Create the session cookie. This will also verify the ID token in the process.
	// The session cookie will have the same claims as the ID token.
	// To only allow session cookie setting on recent sign-in, auth_time in ID token
	// can be checked to ensure user was recently signed in before creating a session cookie.
	cookie, err := a.firebase.SessionCookie(r.Context(), body.IDToken, expiresIn)
	if err != nil {
		log.Printf("%#v", err)
		http.Error(w, "Failed to create a session cookie", 500)
		return
	}
	// Set cookie policy for session cookie.
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    cookie,
		MaxAge:   int(expiresIn.Seconds()),
		HttpOnly: true,
		Secure:   true,
		Path:     "/",
	})
	w.Write([]byte(`{"status": "success"}`))
}
