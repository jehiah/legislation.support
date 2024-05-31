package datastore

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/jehiah/legislation.support/internal/account"
	"github.com/jehiah/legislation.support/internal/legislature"
	"github.com/jehiah/legislation.support/internal/resolvers"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"
)

type Datastore struct {
	firestore *firestore.Client
}

func New(firestore *firestore.Client) *Datastore {
	return &Datastore{firestore: firestore}
}

func (db *Datastore) GetProfile(ctx context.Context, ID account.ProfileID) (*account.Profile, error) {
	if !account.IsValidProfileID(ID) {
		return nil, nil
	}
	dsnap, err := db.firestore.Collection("profiles").Doc(string(ID)).Get(ctx)
	if err != nil {
		if IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	if !dsnap.Exists() {
		return nil, nil
	}
	var p account.Profile
	err = dsnap.DataTo(&p)
	return &p, err
}

func (db *Datastore) GetProfiles(ctx context.Context, UID account.UID) ([]account.Profile, error) {
	query := db.firestore.Collection("profiles").Where("UID", "==", string(UID)).OrderBy("Name", firestore.Asc).Limit(100)
	// ref := db.firestore.Collection(fmt.Sprintf("users/%s/profiles", UID))
	iter := query.Documents(ctx)
	defer iter.Stop()
	var out []account.Profile
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		var p account.Profile
		err = doc.DataTo(&p)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

// GetStaleBills gets bills in an active session that have not been checked recently
func (db *Datastore) GetStaleBills(ctx context.Context, limit int) ([]legislature.Legislation, error) {
	target := time.Hour * 6
	now := time.Now().UTC()
	cutoff := now.Add(-1 * target)
	iter := db.firestore.CollectionGroup("bills").Where(
		"LastChecked", "<", cutoff).Where(
		"Session.EndYear", ">=", now.Year()).Limit(limit).Documents(ctx)
	defer iter.Stop()
	var out []legislature.Legislation
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		var o legislature.Legislation
		err = doc.DataTo(&o)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, nil
}

func (db *Datastore) GetRecentBills(ctx context.Context, limit int) ([]legislature.Legislation, error) {
	if limit == 0 || limit > 1000 {
		limit = 20
	}
	iter := db.firestore.CollectionGroup("bills").OrderBy("Added", firestore.Desc).Limit(limit).Documents(ctx)
	defer iter.Stop()
	var out []legislature.Legislation
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		var o legislature.Legislation
		err = doc.DataTo(&o)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, nil
}

// GetAllBills iterates over all bills
func (db *Datastore) GetAllBills(ctx context.Context, callback func(l legislature.Legislation) error) error {
	iter := db.firestore.CollectionGroup("bills").Documents(ctx)
	defer iter.Stop()
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return err
		}
		var o legislature.Legislation
		err = doc.DataTo(&o)
		if err != nil {
			return err
		}
		err = callback(o)
		if err != nil {
			return err
		}
	}
	return nil
}

// func (db *Datastore) GetBookmarks(ctx context.Context, p account.ProfileID) ([]account.Bookmark, error) {
// 	query := db.firestore.Collection("profiles").Doc(string(p)).Collection("bookmarks").Limit(1000) //.OrderBy("Name", firestore.Asc).Limit(100)
// 	// ref := db.firestore.Collection(fmt.Sprintf("users/%s/profiles", UID))
// 	iter := query.Documents(ctx)
// 	defer iter.Stop()
// 	var out []account.Bookmark
// 	for {
// 		doc, err := iter.Next()
// 		if err == iterator.Done {
// 			break
// 		}
// 		if err != nil {
// 			return nil, err
// 		}
// 		var p account.Bookmark
// 		err = doc.DataTo(&p)
// 		if err != nil {
// 			return nil, err
// 		}
// 		out = append(out, p)
// 	}
// 	return out, nil
// }

func (db *Datastore) CreateProfile(ctx context.Context, p account.Profile) error {
	p.LastModified = time.Now().UTC()
	log.Printf("creating profile %#v", p)
	_, err := db.firestore.Collection("profiles").Doc(string(p.ID)).Create(ctx, p)
	return err
}

func (db *Datastore) UpdateProfile(ctx context.Context, p account.Profile) error {
	p.LastModified = time.Now().UTC()
	log.Printf("updated profile %#v", p)
	_, err := db.firestore.Collection("profiles").Doc(string(p.ID)).Set(ctx, p)
	return err
}

func (db *Datastore) SaveBookmark(ctx context.Context, p account.ProfileID, b account.Bookmark) error {
	b.LastModified = time.Now().UTC()
	_, err := db.firestore.Collection("profiles").Doc(string(p)).Collection("bookmarks").Doc(b.Key()).Create(ctx, b)
	return err
}

func (db *Datastore) UpdateBookmark(ctx context.Context, p account.ProfileID, b account.Bookmark) error {
	b.LastModified = time.Now().UTC()
	_, err := db.firestore.Collection("profiles").Doc(string(p)).Collection("bookmarks").Doc(b.Key()).Set(ctx, b)
	return err
}

func (db *Datastore) DeleteBookmark(ctx context.Context, p account.ProfileID, b legislature.BodyID, l legislature.LegislationID) error {
	k := account.BookmarkKey(b, l)
	_, err := db.firestore.Collection("profiles").Doc(string(p)).Collection("bookmarks").Doc(k).Delete(ctx)
	return err
}

func (db *Datastore) GetBookmark(ctx context.Context, p account.ProfileID, key string) (*account.Bookmark, error) {
	if !account.IsValidProfileID(p) {
		return nil, nil
	}
	dsnap, err := db.firestore.Collection("profiles").Doc(string(p)).Collection("bookmarks").Doc(key).Get(ctx)
	if err != nil {
		if IsNotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	if !dsnap.Exists() {
		return nil, nil
	}
	var b account.Bookmark
	err = dsnap.DataTo(&b)
	return &b, err
}

func (app *Datastore) UpdateBill(ctx context.Context, a, b legislature.Legislation) error {

	changes := legislature.CalculateSponsorChanges(a, b)
	if len(changes) > 0 {
		log.Printf("debug changes %s %s %#v", b.Body, b.ID, changes)
		changesArray := make([]interface{}, len(changes))
		for i := range changes {
			changesArray[i] = changes[i]
		}
		_, err := app.firestore.Collection("bodies").Doc(string(b.Body)).Collection("changes").Doc(string(b.ID)).Update(ctx, []firestore.Update{
			{Path: "Sponsors", Value: firestore.ArrayUnion(changesArray...)},
		})
		if err != nil && IsNotFound(err) {
			_, err = app.firestore.Collection("bodies").Doc(string(b.Body)).Collection("changes").Doc(string(b.ID)).Set(ctx, legislature.Changes{
				Sponsors: changes,
			})
			if err != nil {
				log.Printf("err setting changes %s", err)
			}
		} else if err != nil {
			log.Printf("err setting changes %s", err)
		}
	}

	b.LastChecked = time.Now().UTC()
	_, err := app.firestore.Collection("bodies").Doc(string(a.Body)).Collection("bills").Doc(string(a.ID)).Set(ctx, b)
	return err
}

func (db *Datastore) SaveBill(ctx context.Context, b legislature.Legislation) error {
	b.Added = time.Now().UTC()
	b.LastChecked = time.Now().UTC()
	_, err := db.firestore.Collection("bodies").Doc(string(b.Body)).Collection("bills").Doc(string(b.ID)).Create(ctx, b)
	if IsAlreadyExists(err) {
		var bb *legislature.Legislation
		bb, err = db.GetBill(ctx, b.Body, b.ID)
		if err != nil {
			return err
		}
		bb.IntroducedDate = b.IntroducedDate
		bb.LastModified = b.LastModified
		bb.LastChecked = time.Now().UTC()
		bb.Session = b.Session
		bb.Title = b.Title
		bb.Description = b.Description
		bb.Summary = b.Summary
		bb.Status = b.Status
		bb.SameAs = b.SameAs
		bb.SubstitutedBy = b.SubstitutedBy

		changes := legislature.CalculateSponsorChanges(*bb, b)
		if len(changes) > 0 {
			log.Printf("debug changes %s %s %#v", b.Body, b.ID, changes)
			changesArray := make([]interface{}, len(changes))
			for i := range changes {
				changesArray[i] = changes[i]
			}
			_, err := db.firestore.Collection("bodies").Doc(string(b.Body)).Collection("changes").Doc(string(b.ID)).Update(ctx, []firestore.Update{
				{Path: "Sponsors", Value: firestore.ArrayUnion(changesArray...)},
			})
			if err != nil && IsNotFound(err) {
				_, err = db.firestore.Collection("bodies").Doc(string(b.Body)).Collection("changes").Doc(string(b.ID)).Set(ctx, legislature.Changes{
					Sponsors: changes,
				})
				if err != nil {
					log.Printf("err setting changes %s", err)
				}
			} else if err != nil {
				log.Printf("err setting changes %s", err)
			}
		}

		// TODO: more
		_, err = db.firestore.Collection("bodies").Doc(string(b.Body)).Collection("bills").Doc(string(b.ID)).Set(ctx, *bb)

	}
	// TODO: handle duplicates
	return err
}

func (db *Datastore) GetBill(ctx context.Context, body legislature.BodyID, id legislature.LegislationID) (*legislature.Legislation, error) {
	dsnap, err := db.firestore.Collection("bodies").Doc(string(body)).Collection("bills").Doc(string(id)).Get(ctx)
	if err != nil {
		return nil, err
	}
	var l legislature.Legislation
	err = dsnap.DataTo(&l)
	return &l, err
}

func (db *Datastore) GetProfileBookmarks(ctx context.Context, profileID account.ProfileID) (account.Bookmarks, error) {
	var out account.Bookmarks
	query := db.firestore.Collection(fmt.Sprintf("profiles/%s/bookmarks", profileID)).Limit(5000)
	iter := query.Documents(ctx)
	defer iter.Stop()
	bills := []*firestore.DocumentRef{}
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		var b account.Bookmark
		err = doc.DataTo(&b)
		if err != nil {
			return nil, err
		}
		body := resolvers.Bodies[b.BodyID]
		b.Body = &body
		out = append(out, b)
		bills = append(bills, db.firestore.Collection("bodies").Doc(string(b.BodyID)).Collection("bills").Doc(string(b.LegislationID)))
	}

	docs, err := db.firestore.GetAll(ctx, bills)
	if err != nil {
		return nil, err
	}
	for i, d := range docs {
		var l legislature.Legislation
		err = d.DataTo(&l)
		if err != nil {
			return nil, err
		}
		out[i].Legislation = &l

		if l.SameAs != "" {
			body := resolvers.Bodies[out[i].Body.Bicameral]
			out[i].BicameralBody = &body
		}

	}

	return out, nil
}

type BookmarkChanges struct {
	account.Bookmark
	legislature.Changes
	SameAsChanges legislature.Changes
}

// GetProfileChanges returns all bills regardless of if any chagnes were detected
func (db *Datastore) GetProfileChanges(ctx context.Context, profileID account.ProfileID) ([]BookmarkChanges, error) {
	var out []BookmarkChanges
	var working []*BookmarkChanges
	query := db.firestore.Collection(fmt.Sprintf("profiles/%s/bookmarks", profileID)).Limit(5000)
	iter := query.Documents(ctx)
	defer iter.Stop()
	var refs []*firestore.DocumentRef
	var target []any
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return out, err
		}
		var b account.Bookmark
		err = doc.DataTo(&b)
		if err != nil {
			return out, err
		}
		body := resolvers.Bodies[b.BodyID]
		b.Body = &body
		if body.Bicameral != "" {
			body := resolvers.Bodies[body.Bicameral]
			b.BicameralBody = &body
		}
		bc := &BookmarkChanges{Bookmark: b}
		working = append(working, bc)
		refs = append(refs, db.firestore.Collection("bodies").Doc(string(b.BodyID)).Collection("bills").Doc(string(b.LegislationID)))
		target = append(target, &bc.Legislation)
		refs = append(refs, db.firestore.Collection("bodies").Doc(string(b.BodyID)).Collection("changes").Doc(string(b.LegislationID)))
		target = append(target, &bc.Changes)
	}

	docs, err := db.firestore.GetAll(ctx, refs)
	if err != nil {
		return out, err
	}
	for i, d := range docs {
		err = d.DataTo(target[i])
		if err != nil && !IsNotFound(err) {
			return out, err
		}
	}

	refs = []*firestore.DocumentRef{}
	target = []any{}

	for _, b := range working {
		if b.Legislation.SameAs != "" {
			refs = append(refs, db.firestore.Collection("bodies").Doc(string(b.Body.Bicameral)).Collection("changes").Doc(string(b.Legislation.SameAs)))
			target = append(target, &b.SameAsChanges)
		}
	}

	if len(refs) > 0 {
		docs, err := db.firestore.GetAll(ctx, refs)
		if err != nil {
			return out, err
		}
		for i, d := range docs {
			err = d.DataTo(target[i])
			if err != nil && !IsNotFound(err) {
				return out, err
			}
		}
	}

	out = make([]BookmarkChanges, len(working))
	for i, bc := range working {
		out[i] = *bc
	}
	return out, nil

}

func (db *Datastore) GetChanges(ctx context.Context, body legislature.BodyID, id legislature.LegislationID) (legislature.Changes, error) {
	var r legislature.Changes
	dsnap, err := db.firestore.Collection("bodies").Doc(string(body)).Collection("changes").Doc(string(id)).Get(ctx)
	if err != nil && IsNotFound(err) {
		return r, nil
	} else if err != nil {
		return r, err
	}
	err = dsnap.DataTo(&r)
	return r, err
}
