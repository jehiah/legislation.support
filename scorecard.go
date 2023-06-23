package main

import (
	"net/http"
	"sort"
	"strings"

	"github.com/jehiah/legislation.support/internal/account"
	"github.com/jehiah/legislation.support/internal/resolvers"
	"github.com/jehiah/legislation.support/internal/resolvers/nyc"
	"github.com/jehiah/legislator/db"
	log "github.com/sirupsen/logrus"
)

type Score struct {
	// Score   int
	Status  string
	Desired bool
}

func (s Score) Score() int {
	if s.Desired {
		switch strings.ToLower(s.Status) {
		case "affirmative", "sponsor":
			return 1
		case "negative":
			return -1
		}
		return 0
	}
	switch strings.ToLower(s.Status) {
	case "affirmative", "sponsor":
		return -1
	case "negative":
		return 1
	default:
		return 0
	}
}

func (s Score) CSS() string {
	return strings.ToLower(s.Status)
}

type Column struct {
	Legislation *db.Legislation
	Bookmark    account.Bookmark
	Scores      []Score
}
type Columns []Column

func (c Column) PercentCorrect() float64 {
	have := 0
	for _, s := range c.Scores {
		if s.Score() == 1 {
			have++
		}
	}
	return (float64(have) / float64(len(c.Scores))) * 100
}

func (c Columns) PercentCorrect(idx int) float64 {
	have := 0
	for _, cc := range c {
		if cc.Scores[idx].Score() == 1 {
			have++
		}
	}
	return (float64(have) / float64(len(c))) * 100
}

// Scorecard builds a scorecard for the tracked bills
func (a *App) Scorecard(w http.ResponseWriter, r *http.Request, profileID account.ProfileID) {
	t := newTemplate(a.templateFS, "scorecard_nyc.html")
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

	type Page struct {
		Page      string
		Title     string
		UID       account.UID
		Profile   account.Profile
		EditMode  bool
		Bookmarks []account.Bookmark
		People    []db.Person
		Columns   Columns
	}
	body := Page{
		Title:    profile.Name + " scorecard (legislation.support)",
		Profile:  *profile,
		EditMode: uid == profile.UID,
		UID:      uid,
	}
	b, err := a.GetProfileBookmarks(ctx, profileID)
	if err != nil {
		log.WithField("uid", uid).WithField("profileID", profileID).Errorf("%#v", err)
		a.WebInternalError500(w, "")
		return
	}
	for _, bb := range b {
		if bb.Legislation.Session.Active() {
			if bb.Legislation.Body == resolvers.NYCCouncil.ID {
				body.Bookmarks = append(body.Bookmarks, bb)
			}
		}
	}

	nycResolver := resolvers.Resolvers.Find(resolvers.NYCCouncil.ID).(*nyc.NYC)
	body.People, err = nycResolver.ActivePeople(ctx)
	if err != nil {
		a.WebInternalError500(w, err.Error())
		return
	}
	// data cleanup
	for i, p := range body.People {
		body.People[i].FullName = strings.TrimSpace(p.FullName)
	}

	// remove the public advocate
	for i, p := range body.People {
		switch p.ID {
		case 7780: // public advocate
			body.People = append(body.People[:i], body.People[i+1:]...)
			break
		}
	}

	sort.Sort(account.SortedBookmarks(body.Bookmarks))
	for _, l := range body.Bookmarks {
		raw, err := nycResolver.Raw(ctx, l.Legislation)
		if err != nil {
			a.WebInternalError500(w, err.Error())
			return
		}
		scores := make(map[string]string)
		for _, sponsor := range raw.Sponsors {
			scores[strings.TrimSpace(sponsor.FullName)] = "Sponsor"
		}
		for _, h := range raw.History {
			for _, v := range h.Votes {
				scores[strings.TrimSpace(v.FullName)] = v.Vote
			}
		}
		c := Column{
			Legislation: raw,
			Bookmark:    l,
		}
		// TODO: determine if we desire yes/now
		for _, p := range body.People {
			c.Scores = append(c.Scores, Score{Status: scores[p.FullName], Desired: true})
		}
		body.Columns = append(body.Columns, c)
	}

	// log.Printf("bookmarks %#v", body.Bookmarks)

	err = t.ExecuteTemplate(w, "scorecard_nyc.html", body)
	if err != nil {
		log.WithField("uid", uid).Error(err)
		a.WebInternalError500(w, "")
	}
}
