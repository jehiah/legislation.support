package main

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jehiah/legislation.support/internal/account"
	"github.com/jehiah/legislation.support/internal/apiresponse"
	"github.com/jehiah/legislation.support/internal/datastore"
	"github.com/jehiah/legislation.support/internal/legislature"
	"github.com/jehiah/legislation.support/internal/metadatasites"
	"github.com/jehiah/legislation.support/internal/resolvers"
	log "github.com/sirupsen/logrus"
)

type ProfileMetadata struct {
	account.Profile

	SupportedBills int
	OpposedBills   int
	ArchivedBills  int
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
		Profiles []ProfileMetadata
	}
	body := Page{
		Title: "legislation.support",
		UID:   uid,
	}

	profiles, err := a.GetProfiles(ctx, uid)
	if err != nil {
		log.Print(err)
		a.WebInternalError500(w, "")
		return
	}

	for _, p := range profiles {
		profile := ProfileMetadata{
			Profile: p,
		}
		b, err := a.GetProfileBookmarks(ctx, p.ID)
		if err != nil {
			log.WithField("uid", uid).WithField("profileID", p.ID).Errorf("%#v", err)
			a.WebInternalError500(w, "")
			return
		}
		for _, bb := range b {
			if bb.LastModified.After(profile.LastModified) {
				profile.LastModified = bb.LastModified
			}
			if bb.Legislation.Session.Active() {
				if bb.Oppose {
					profile.OpposedBills++
				} else {
					profile.SupportedBills++
				}
			} else {
				profile.ArchivedBills++
			}
		}
		body.Profiles = append(body.Profiles, profile)
	}

	sort.Slice(body.Profiles, func(i, j int) bool {
		return body.Profiles[i].LastModified.After(body.Profiles[j].LastModified)
	})

	t := newTemplate(a.templateFS, "profiles.html")
	err = t.ExecuteTemplate(w, "profiles.html", body)
	if err != nil {
		log.Print(err)
		a.WebInternalError500(w, "")
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

func (a *App) Profile(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	profileID := account.ProfileID(r.PathValue("profile"))
	if !account.IsValidProfileID(profileID) {
		log.Printf("invalid profile %q", profileID)
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
	a.ShowProfile(w, ctx, r, uid, profile, Message{})
}

func (a *App) ShowProfile(w http.ResponseWriter, ctx context.Context, r *http.Request, uid account.UID, profile *account.Profile, message Message) {
	templateName := "profile.html"
	t := newTemplate(a.templateFS, "profile.html")
	profileID := profile.ID
	r.ParseForm()

	type Page struct {
		Page              string
		Title             string
		Message           Message
		UID               account.UID
		Profile           account.Profile
		EditMode          bool
		SelectedTag       string
		Bookmarks         account.Bookmarks
		ArchivedBookmarks account.Bookmarks
	}
	body := Page{
		Message:           message,
		Title:             profile.Name + " (legislation.support)",
		Profile:           *profile,
		EditMode:          uid == profile.UID,
		UID:               uid,
		SelectedTag:       r.Form.Get("tag"),
		Bookmarks:         make(account.Bookmarks, 0),
		ArchivedBookmarks: make(account.Bookmarks, 0),
	}

	if body.EditMode {
		templateName = "profile_edit.html"
		t = newTemplate(a.templateFS, "profile_edit.html")
	}
	b, err := a.GetProfileBookmarks(ctx, profileID)
	if err != nil {
		log.WithField("uid", uid).WithField("profileID", profileID).Errorf("%s", err)
		a.WebInternalError500(w, "")
		return
	}
	for _, bb := range b {
		if bb.Legislation.Session.Active() {
			body.Bookmarks = append(body.Bookmarks, bb)
		} else {
			body.ArchivedBookmarks = append(body.ArchivedBookmarks, bb)
		}
	}

	if body.SelectedTag != "" {
		var hasTag bool
		for _, t := range body.Bookmarks.DisplayTags() {
			if t.Tag == body.SelectedTag {
				hasTag = true
				break
			}
		}
		if !hasTag {
			http.Redirect(w, r, profile.Link(), 302)
			return
		}
	}

	sort.Sort(account.SortedBookmarks(body.Bookmarks))
	sort.Sort(account.SortedBookmarks(body.ArchivedBookmarks))
	// log.Printf("bookmarks %#v", body.Bookmarks)

	err = t.ExecuteTemplate(w, templateName, body)
	if err != nil {
		log.WithField("uid", uid).Errorf("error %s", err)
		a.WebInternalError500(w, "")
	}
}

type Message struct {
	Success string `json:"success,omitempty"`
	Error   string `json:"error,omitempty"`
}

// ProfilePost handles the add of a new URL to a profile, or update of a profile
// POST /data/profile
func (a *App) ProfilePost(w http.ResponseWriter, r *http.Request) {
	r.ParseMultipartForm(1 << 20) // 2Mb
	ctx := r.Context()
	uid := a.User(r)

	profileID := account.ProfileID(r.Form.Get("profile_id"))
	logFields := log.Fields{"uid": uid, "profileID": profileID}

	profile, err := a.GetProfile(ctx, profileID)
	if err != nil {
		log.WithContext(ctx).WithFields(logFields).Errorf("%#v", err)
		apiresponse.InternalError500(w)
		return
	}
	if profile == nil {
		log.WithContext(ctx).WithFields(logFields).Warnf("not found")
		apiresponse.NotFound404(w)
		return
	}

	if !profile.HasAccess(uid) {
		a.WebPermissionError403(w, "")
		return
	}

	switch {
	case strings.TrimSpace(r.Form.Get("legislation_url")) != "":
		changes := a.ProfilePostURL(ctx, profileID, r)
		log.Printf("changes %#v", changes)
		var m Message
		for _, c := range changes {
			if c == nil {
				continue
			}
			if c.Error != "" {
				m.Error += c.Error + "\n"
			} else {
				if c.New {
					m.Success += fmt.Sprintf("Added %s %s %s\n", c.Body.Name, c.Legislation.DisplayID, c.Legislation.Title)
				} else {
					m.Success += fmt.Sprintf("Updated %s %s %s\n", c.Body.Name, c.Legislation.DisplayID, c.Legislation.Title)
				}
			}
		}
		apiresponse.OK200(w, m)
		return
	case strings.TrimSpace(r.Form.Get("name")) != "":
		err = a.ProfileEdit(ctx, *profile, r)
	}

	if err != nil {
		log.WithContext(ctx).WithFields(logFields).Errorf("%#v", err)
		apiresponse.InternalError500(w)
		return
	}
	apiresponse.OK200(w, nil)
}

func (a *App) ProfileEdit(ctx context.Context, p account.Profile, r *http.Request) error {
	p.Name = strings.TrimSpace(r.Form.Get("name"))
	if p.Name == "" {
		return fmt.Errorf("name required")
	}
	p.Description = strings.TrimSpace(r.Form.Get("description"))
	p.Private = r.Form.Get("private") == "on"
	p.HideDistrict = r.Form.Get("hide_district") == "on"
	p.HideSupportOppose = r.Form.Get("hide_support_oppose") == "on"
	p.HideBillStatus = r.Form.Get("hide_bill_status") == "on"
	return a.UpdateProfile(ctx, p)
}

func (a *App) ProfilePostURL(ctx context.Context, profileID account.ProfileID, r *http.Request) []*BookmarkChange {
	uid := a.User(r)
	input := strings.Fields(strings.TrimSpace(r.Form.Get("legislation_url")))
	output := make([]*BookmarkChange, len(input))

	var wg sync.WaitGroup
	for i, legUrl := range input {
		i := i
		legUrl := legUrl
		wg.Add(1)
		go func() {
			defer wg.Done()
			o := &BookmarkChange{
				URL: legUrl,
			}
			output[i] = o
			o.record(func() error {
				var u, matchedURL *url.URL
				u, err := url.Parse(legUrl)
				if err != nil {
					return err
				}
				logFields := log.Fields{"uid": uid, "profileID": profileID, "legislation_url": u.String()}
				log.WithContext(ctx).WithFields(logFields).Infof("parsed URL host:%s", u.Hostname())
				var bill *legislature.Legislation
				matchedURL, err = metadatasites.Lookup(r.Context(), u)
				if err != nil {
					log.WithContext(ctx).WithFields(logFields).Errorf("metadatasites.Lookup error %#v", err)
				}
				if matchedURL.Hostname() != u.Hostname() {
					log.WithContext(ctx).WithFields(logFields).Infof("metadatasites found URL %q", matchedURL)
				}
				bill, err = resolvers.Lookup(r.Context(), matchedURL)
				if err != nil {
					return err
				}
				if bill == nil {
					return fmt.Errorf("legislation matching url %q not found", u)

				}
				body := resolvers.Bodies[bill.Body]

				// Save refreshes a bill as well
				err = a.SaveBill(ctx, *bill)
				if err != nil {
					return err
				}

				o.Bookmark, err = a.GetBookmark(ctx, profileID, account.BookmarkKey(bill.Body, bill.ID))
				if err != nil {
					return err
				}
				if o.Bookmark != nil {
					o.Bookmark.Notes = strings.TrimSpace(r.Form.Get("notes"))
					o.Bookmark.Tags = strings.Fields(strings.TrimSpace(r.Form.Get("tags")))
					o.Bookmark.Oppose = r.Form.Get("support") == "👎"

					o.Legislation = bill
					o.Body = &body

					// update
					return a.UpdateBookmark(ctx, profileID, *o.Bookmark)
				}

				o.Bookmark = &account.Bookmark{
					UID:           uid,
					BodyID:        bill.Body,
					LegislationID: bill.ID,
					Oppose:        r.Form.Get("support") == "👎",
					Created:       time.Now().UTC(),
					Notes:         strings.TrimSpace(r.Form.Get("notes")),
					Tags:          strings.Fields(strings.TrimSpace(r.Form.Get("tags"))),

					Legislation: bill,
					Body:        &body,
				}
				err = a.SaveBookmark(ctx, profileID, *o.Bookmark)
				if err != nil && !datastore.IsAlreadyExists(err) {
					return err
				}
				return nil
			}())
		}()
	}
	wg.Wait()
	return output

}

// ProfileRemove removes a bookmark from a profile
func (a *App) ProfileRemove(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(2 << 20) // 2Mb
	if err != nil {
		log.Printf("err parsing form %s", err)
	}
	ctx := r.Context()
	uid := a.User(r)
	profileID := account.ProfileID(r.PostForm.Get("profile_id"))
	body, legID := legislature.BodyID(r.PostForm.Get("body_id")), legislature.LegislationID(r.PostForm.Get("legislation_id"))
	logFields := log.Fields{"uid": uid, "profileID": profileID, "body": body, "legislation_id": legID}

	profile, err := a.GetProfile(ctx, profileID)
	if err != nil {
		log.WithContext(ctx).WithFields(logFields).Errorf("%#v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	if profile == nil {
		http.Error(w, "Not Found", 404)
		return
	}

	if !profile.HasAccess(uid) {
		http.Error(w, "Permission Denied.", 403)
		return
	}

	err = a.DeleteBookmark(ctx, profileID, body, legID)
	if err != nil {
		log.WithContext(ctx).WithFields(logFields).Errorf("%#v", err)
		http.Error(w, err.Error(), 500)
		return
	}
	http.Redirect(w, r, profile.Link(), 302)
	return
}
