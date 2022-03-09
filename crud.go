package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/jehiah/legislation.support/internal/account"
	"github.com/jehiah/legislation.support/internal/legislature"
	"google.golang.org/api/iterator"
)

func (a *App) GetProfile(ctx context.Context, ID string) (*account.Profile, error) {
	dsnap, err := a.firestore.Collection("profiles").Doc(ID).Get(ctx)
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

func (a *App) GetProfiles(ctx context.Context, UID string) ([]account.Profile, error) {
	query := a.firestore.Collection("profiles").Where("UID", "==", UID)
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

func (a *App) CreateProfile(ctx context.Context, p account.Profile) error {
	p.LastModified = time.Now().UTC()
	log.Printf("creating profile %#v", p)
	_, err := a.firestore.Collection("profiles").Doc(p.ID).Create(ctx, p)
	return err
}

func (a *App) GetProfileBills(ctx context.Context, profileID string) ([]legislature.Legislation, error) {
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
