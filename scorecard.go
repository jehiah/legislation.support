package main

import (
	"errors"
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
	ctx := r.Context()
	r.ParseForm()

	templateName := "scorecard.html"
	if r.Form.Get("view") == "people" {
		templateName = "scorecard_people.html"
	}
	t := newTemplate(a.templateFS, templateName)

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
		PersonWhipCounts []legislature.PersonWhipCount
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

	if t := r.Form.Get("tag"); t != "" {
		bookmarks = bookmarks.FilterTag(t)
		pageBody.Title = fmt.Sprintf("%s %s Scorecard %s", profile.Name, body.Name, t)
	}

	sort.Sort(account.SortedBookmarks(bookmarks))
	var scorable []legislature.Scorable
	for _, b := range bookmarks {
		scorable = append(scorable, b)
	}
	pageBody.Scorecard, err = resolvers.Resolvers.Find(body.ID).Scorecard(ctx, scorable)
	if err != nil {
		log.WithFields(fields).Errorf("%#v, %#v", err, errors.Unwrap(err))
		a.WebInternalError500(w, "")
		return
	}

	for i, p := range pageBody.People {
		pageBody.PersonWhipCounts = append(pageBody.PersonWhipCounts, legislature.PersonWhipCount{
			ScorecardPerson: p,
			WhipCount:       pageBody.Scorecard.WhipCount(i),
		})
	}

	sort.Slice(pageBody.PersonWhipCounts, func(i, j int) bool {
		return pageBody.PersonWhipCounts[i].WhipCount.Percent() > pageBody.PersonWhipCounts[j].WhipCount.Percent()
	})

	// log.Printf("bookmarks %#v", body.Bookmarks)

	err = t.ExecuteTemplate(w, templateName, pageBody)
	if err != nil {
		log.WithFields(fields).Error(err)
		a.WebInternalError500(w, "")
	}
}
