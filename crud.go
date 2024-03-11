package main

import (
	"context"
	"fmt"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/jehiah/legislation.support/internal/account"
	"github.com/jehiah/legislation.support/internal/legislature"
	"github.com/jehiah/legislation.support/internal/resolvers"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/iterator"
)

func (a *App) GetProfile(ctx context.Context, ID account.ProfileID) (*account.Profile, error) {
	if !account.IsValidProfileID(ID) {
		return nil, nil
	}
	dsnap, err := a.firestore.Collection("profiles").Doc(string(ID)).Get(ctx)
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

func (a *App) GetProfiles(ctx context.Context, UID account.UID) ([]account.Profile, error) {
	query := a.firestore.Collection("profiles").Where("UID", "==", string(UID)).OrderBy("Name", firestore.Asc).Limit(100)
	// ref := a.firestore.Collection(fmt.Sprintf("users/%s/profiles", UID))
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

func (a *App) GetRecentBills(ctx context.Context, limit int) ([]legislature.Legislation, error) {
	if limit == 0 || limit > 1000 {
		limit = 20
	}
	iter := a.firestore.CollectionGroup("bills").OrderBy("Added", firestore.Desc).Limit(limit).Documents(ctx)
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

// func (a *App) GetBookmarks(ctx context.Context, p account.ProfileID) ([]account.Bookmark, error) {
// 	query := a.firestore.Collection("profiles").Doc(string(p)).Collection("bookmarks").Limit(1000) //.OrderBy("Name", firestore.Asc).Limit(100)
// 	// ref := a.firestore.Collection(fmt.Sprintf("users/%s/profiles", UID))
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

func (a *App) CreateProfile(ctx context.Context, p account.Profile) error {
	p.LastModified = time.Now().UTC()
	log.Printf("creating profile %#v", p)
	_, err := a.firestore.Collection("profiles").Doc(string(p.ID)).Create(ctx, p)
	return err
}

func (a *App) UpdateProfile(ctx context.Context, p account.Profile) error {
	p.LastModified = time.Now().UTC()
	log.Printf("updated profile %#v", p)
	_, err := a.firestore.Collection("profiles").Doc(string(p.ID)).Set(ctx, p)
	return err
}

func (a *App) SaveBookmark(ctx context.Context, p account.ProfileID, b account.Bookmark) error {
	b.LastModified = time.Now().UTC()
	_, err := a.firestore.Collection("profiles").Doc(string(p)).Collection("bookmarks").Doc(b.Key()).Create(ctx, b)
	return err
}

func (a *App) UpdateBookmark(ctx context.Context, p account.ProfileID, b account.Bookmark) error {
	b.LastModified = time.Now().UTC()
	_, err := a.firestore.Collection("profiles").Doc(string(p)).Collection("bookmarks").Doc(b.Key()).Set(ctx, b)
	return err
}

func (a *App) DeleteBookmark(ctx context.Context, p account.ProfileID, b legislature.BodyID, l legislature.LegislationID) error {
	k := account.BookmarkKey(b, l)
	_, err := a.firestore.Collection("profiles").Doc(string(p)).Collection("bookmarks").Doc(k).Delete(ctx)
	return err
}

func (a *App) GetBookmark(ctx context.Context, p account.ProfileID, key string) (*account.Bookmark, error) {
	if !account.IsValidProfileID(p) {
		return nil, nil
	}
	dsnap, err := a.firestore.Collection("profiles").Doc(string(p)).Collection("bookmarks").Doc(key).Get(ctx)
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

func (a *App) UpdateBill(ctx context.Context, b legislature.Legislation) error {

	b.LastChecked = time.Now().UTC()
	_, err := a.firestore.Collection("bodies").Doc(string(b.Body)).Collection("bills").Doc(string(b.ID)).Set(ctx, b)
	// TODO: handle duplicates
	return err
}

func (a *App) SaveBill(ctx context.Context, b legislature.Legislation) error {
	b.Added = time.Now().UTC()
	b.LastChecked = time.Now().UTC()
	_, err := a.firestore.Collection("bodies").Doc(string(b.Body)).Collection("bills").Doc(string(b.ID)).Create(ctx, b)
	if IsAlreadyExists(err) {
		var bb *legislature.Legislation
		bb, err = a.GetBill(ctx, b.Body, b.ID)
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
		// TODO: more
		_, err = a.firestore.Collection("bodies").Doc(string(b.Body)).Collection("bills").Doc(string(b.ID)).Set(ctx, *bb)

	}
	// TODO: handle duplicates
	return err
}

func (a *App) GetBill(ctx context.Context, body legislature.BodyID, id legislature.LegislationID) (*legislature.Legislation, error) {
	dsnap, err := a.firestore.Collection("bodies").Doc(string(body)).Collection("bills").Doc(string(id)).Get(ctx)
	if err != nil {
		return nil, err
	}
	var l legislature.Legislation
	err = dsnap.DataTo(&l)
	return &l, err
}

// func (a *App) GetProfileBills(ctx context.Context, profileID string) ([]legislature.Legislation, error) {
// }

func (a *App) GetProfileBookmarks(ctx context.Context, profileID account.ProfileID) (account.Bookmarks, error) {
	var out account.Bookmarks
	query := a.firestore.Collection(fmt.Sprintf("profiles/%s/bookmarks", profileID)).Limit(5000)
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
		bills = append(bills, a.firestore.Collection("bodies").Doc(string(b.BodyID)).Collection("bills").Doc(string(b.LegislationID)))
	}

	docs, err := a.firestore.GetAll(ctx, bills)
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
	}

	// if any bills are stale, refresh (some) of them - error should not be fatal
	var wg errgroup.Group
	updated := 0
	for i := range out {
		i := i
		if out[i].Legislation != nil {
			l := out[i].Legislation
			// TODO: don't wait for these results
			if l.IsStale() && updated < 15 {
				updated++
				log.Printf("%q is stale %v", l.ID, l.LastChecked)
				wg.Go(func() error {
					udpatedLeg, err := resolvers.Resolvers.Find(out[i].BodyID).Refresh(ctx, l.ID)
					if err != nil {
						return err
					}
					err = a.UpdateBill(ctx, *udpatedLeg)
					if err != nil {
						return err
					}
					out[i].Legislation = udpatedLeg
					return nil
				})
			}
		}
	}
	if err := wg.Wait(); err != nil {
		log.Printf("err %s", err)
	}

	return out, nil
}
