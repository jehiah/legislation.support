package main

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/jehiah/legislation.support/internal/account"
	"github.com/julienschmidt/httprouter"
)

type SessionRequest struct {
	IDToken string `json:"id_token"`
}

func (a *App) User(r *http.Request) account.UID {
	cookie, err := r.Cookie("session")
	if err != nil {
		return ""
	}
	decoded, err := a.firebase.VerifySessionCookieAndCheckRevoked(r.Context(), cookie.Value)
	if err != nil {
		return ""
	}
	return account.UID(decoded.UID)
}

func (a *App) SignOut(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
	http.SetCookie(w, &http.Cookie{
		Name:   "session",
		Value:  "",
		MaxAge: 0,
	})
	http.Redirect(w, r, "/", 302)
}

func (a *App) NewSession(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
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
		Secure:   !a.devMode,
	})
	w.Write([]byte(`{"status": "success"}`))
}
