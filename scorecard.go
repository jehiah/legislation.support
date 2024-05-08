package main

import (
	"fmt"
	"net/http"
	"sort"

	"github.com/jehiah/legislation.support/internal/account"
	"github.com/jehiah/legislation.support/internal/legislature"
	"github.com/jehiah/legislation.support/internal/resolvers"
	log "github.com/sirupsen/logrus"
)

// Scorecard builds a scorecard for the tracked bills
func (a *App) Scorecard(w http.ResponseWriter, r *http.Request) {
	t := newTemplate(a.templateFS, "scorecard.html")
	ctx := r.Context()

	profileID := account.ProfileID(r.PathValue("profile"))
	if !account.IsValidProfileID(profileID) {
		http.Error(w, "Not Found", 404)
		return
	}
	bodyID := legislature.BodyID(r.PathValue("body"))
	if !resolvers.IsValidBodyID(bodyID) {
		http.Error(w, "Not Found", 404)
		return
	}

	uid := a.User(r)
	fields := log.Fields{"uid": uid, "profileID": profileID, "body": bodyID}

	profile, err := a.GetProfile(ctx, profileID)
	if err != nil {
		log.WithFields(fields).Errorf("%#v", err)
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

	type Page struct {
		Page     string
		Title    string
		UID      account.UID
		Profile  account.Profile
		EditMode bool
		*legislature.Scorecard
		// Bookmarks []account.Bookmark
	}
	b, err := a.GetProfileBookmarks(ctx, profileID)
	if err != nil {
		log.WithFields(fields).Errorf("%#v", err)
		a.WebInternalError500(w, "")
		return
	}

	if bodyID == "" {
		// redirect to a more specific URL
		bodies := b.Bodies()
		if len(bodies) == 0 {
			http.Error(w, "Not Found", 404)
			return
		}
		http.Redirect(w, r, fmt.Sprintf("/%s/scorecard/%s", profileID, bodies[0]), 302)
		return
	}
	body, ok := resolvers.Bodies[bodyID]
	if !ok {
		http.Error(w, "Not Found", 404)
		return
	}

	pageBody := Page{
		Title:    profile.Name + " " + body.Name + " Scorecard",
		Profile:  *profile,
		EditMode: uid == profile.UID,
		UID:      uid,
	}

	// bookmarks := b.Active().Filter(body.ID)
	bookmarks := b.Active().Filter(body.ID, body.Bicameral)

	sort.Sort(account.SortedBookmarks(bookmarks))
	var scorable []legislature.Scorable
	for _, b := range bookmarks {
		scorable = append(scorable, b)
	}
	pageBody.Scorecard, err = resolvers.Resolvers.Find(body.ID).Scorecard(ctx, scorable)
	if err != nil {
		log.WithFields(fields).Errorf("%#v", err)
		a.WebInternalError500(w, "")
		return
	}

	// log.Printf("bookmarks %#v", body.Bookmarks)

	err = t.ExecuteTemplate(w, "scorecard.html", pageBody)
	if err != nil {
		log.WithFields(fields).Error(err)
		a.WebInternalError500(w, "")
	}
}
