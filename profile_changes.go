package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/gorilla/feeds"
	"github.com/jehiah/legislation.support/internal/account"
	"github.com/jehiah/legislation.support/internal/apiresponse"
	"github.com/jehiah/legislation.support/internal/legislature"
	"github.com/jehiah/legislation.support/internal/resolvers"
	log "github.com/sirupsen/logrus"
)

type BookmarkChange struct {
	New   bool
	URL   string
	Error string
	*account.Bookmark
}

func (b *BookmarkChange) record(err error) {
	if err != nil {
		log.Printf("BookmarkChange error %s", err)
		b.Error = err.Error()
	}
}

type Change struct {
	legislature.LegislationID
	*legislature.Body
	account.Bookmark
	legislature.SponsorChange
}

func (a *App) ProfileChanges(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	profileID := account.ProfileID(r.PathValue("profile"))

	if !account.IsValidProfileID(profileID) {
		http.Error(w, "Not Found", 404)
		return
	}
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

	templateName := "profile_changes.html"
	t := newTemplate(a.templateFS, "profile_changes.html")

	type Page struct {
		Page     string
		Title    string
		Message  Message
		UID      account.UID
		Profile  account.Profile
		EditMode bool
		Changes  []Change
	}
	body := Page{
		Title:   profile.Name + " (legislation.support)",
		Profile: *profile,
		UID:     uid,
	}

	b, err := a.GetProfileChanges(ctx, profileID)
	if err != nil {
		log.WithField("uid", uid).WithField("profileID", profileID).Errorf("%s", err)
		a.WebInternalError500(w, "")
		return
	}
	for _, bb := range b {
		if !bb.Legislation.Session.Active() {
			continue
		}

		for _, c := range bb.Changes.Sponsors {
			body.Changes = append(body.Changes, Change{
				LegislationID: bb.LegislationID,
				Body:          bb.Body,
				Bookmark:      bb.Bookmark,
				SponsorChange: c,
			})
		}
		for _, c := range bb.SameAsChanges.Sponsors {
			sameAsBody := resolvers.Bodies[bb.Body.Bicameral]
			body.Changes = append(body.Changes, Change{
				LegislationID: bb.Legislation.SameAs,
				Body:          &sameAsBody,
				Bookmark:      bb.Bookmark,
				SponsorChange: c,
			})
		}
	}

	sort.Slice(body.Changes, func(i, j int) bool {
		return body.Changes[i].SponsorChange.Date.After(body.Changes[j].SponsorChange.Date)
	})

	// sort.Sort(account.SortedBookmarks(body.Bookmarks))
	// log.Printf("bookmarks %#v", body.Bookmarks)

	if strings.HasSuffix(r.URL.Path, "/changes.json") {
		a.ProfileChangesJSON(w, r, *profile, body.Changes)
		return
	}

	err = t.ExecuteTemplate(w, templateName, body)
	if err != nil {
		log.WithField("uid", uid).Error(err)
		a.WebInternalError500(w, "")
	}
}

func (a *App) ProfileChangesJSON(w http.ResponseWriter, r *http.Request, profile account.Profile, changes []Change) {
	feed := &feeds.JSONFeed{
		Title:       profile.Name,
		HomePageUrl: profile.FullLink(),
		FeedUrl:     profile.FullLink() + "/changes.json",
		Version:     "https://jsonfeed.org/version/1",
		// Description: "...",
		// Author:  &feeds.Author{Name: "John Doe", Email: "user@email"},
		// Created: profile.Created,
		// Updated: posts[0].Date,
		// Image: &feeds.Image{
		// 	Link:   "path/to/image.png",
		// 	Width:  960,
		// 	Height: 960,
		// },
	}

	for _, c := range changes {

		feed.Items = append(feed.Items, &feeds.JSONItem{
			Title: fmt.Sprintf("%s Sponsored by %s", c.Legislation.DisplayID, c.SponsorChange.Member.FullName),
			// Link:  &feeds.Link{Href: u.String()},
			Summary:       c.Legislation.Title,
			Author:        &feeds.JSONAuthor{Name: c.SponsorChange.Member.FullName}, // , Email: c.SponsorChange.Member.Email},
			PublishedDate: &c.SponsorChange.Date,
			ModifiedDate:  &c.SponsorChange.Date,
		})
	}

	w.Header().Set("Content-Type", "application/feed+json")
	err := json.NewEncoder(w).Encode(feed)
	if err != nil {
		log.Printf("error writing rss %s", err)
		apiresponse.InternalError500(w)
	}

}
