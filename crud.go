package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/jehiah/legislation.support/internal/account"
	"github.com/jehiah/legislation.support/internal/legislature"
	"google.golang.org/api/iterator"
)

func (a *App) GetProfile(ctx context.Context, ID account.ProfileID) (*account.Profile, error) {
	if !account.IsValidProfileID(ID) {
		return nil, nil
	}
	dsnap, err := a.firestore.Collection("profiles").Doc(string(ID)).Get(ctx)
	if err != nil {
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

func (a *App) GetBookmarks(ctx context.Context, p account.ProfileID) ([]account.Bookmark, error) {
	query := a.firestore.Collection("profiles").Doc(string(p)).Collection("bookmarks").Limit(1000) //.OrderBy("Name", firestore.Asc).Limit(100)
	// ref := a.firestore.Collection(fmt.Sprintf("users/%s/profiles", UID))
	iter := query.Documents(ctx)
	defer iter.Stop()
	var out []account.Bookmark
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		var p account.Bookmark
		err = doc.DataTo(&p)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, nil
}

func (a *App) CreateProfile(ctx context.Context, p account.Profile) error {
	p.LastModified = time.Now().UTC()
	log.Printf("creating profile %#v", p)
	_, err := a.firestore.Collection("profiles").Doc(string(p.ID)).Create(ctx, p)
	return err
}

func (a *App) SaveBookmark(ctx context.Context, p account.ProfileID, b account.Bookmark) error {
	b.LastModified = time.Now().UTC()
	_, err := a.firestore.Collection("profiles").Doc(string(p)).Collection("bookmarks").Doc(b.Key()).Create(ctx, b)
	return err
}

func (a *App) SaveBill(ctx context.Context, b legislature.Legislation) error {
	// b.LastModified = time.Now().UTC()
	_, err := a.firestore.Collection("bodies").Doc(string(b.Body)).Collection("bills").Doc(string(b.ID)).Create(ctx, b)
	// TODO: handle duplicates
	return err
}

// func (a *App) GetProfileBills(ctx context.Context, profileID string) ([]legislature.Legislation, error) {
// }

func (a *App) GetProfileBills(ctx context.Context, profileID account.ProfileID) ([]legislature.Legislation, error) {
	var out []legislature.Legislation
	query := a.firestore.Collection(fmt.Sprintf("profile/%s/bills", profileID))
	iter := query.Documents(ctx)
	defer iter.Stop()
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, err
		}
		var b legislature.Legislation
		err = doc.DataTo(&b)
		if err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, nil
}
