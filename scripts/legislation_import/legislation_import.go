package main

import (
	"context"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jehiah/legislation.support/internal/account"
	"github.com/jehiah/legislation.support/internal/datastore"
	"github.com/jehiah/legislation.support/internal/legislature"
	"github.com/jehiah/legislation.support/internal/metadatasites"
	"github.com/jehiah/legislation.support/internal/resolvers"
	log "github.com/sirupsen/logrus"
)

type Row struct {
	URL     *url.URL
	Support bool
	Tags    []string
	Notes   string
}

func main() {
	inputFile := flag.String("input", "", "input csv file")
	profileID := flag.String("profile", "", "profile name")
	flag.Parse()
	if *inputFile == "" {
		log.Fatal("input file is required")
	}
	ctx := context.Background()
	db := datastore.New(datastore.NewClient(ctx))

	// validate profile
	profile, err := db.GetProfile(ctx, account.ProfileID(*profileID))
	if err != nil || profile == nil {
		log.Fatalf("error getting profile id: %s", err)
	}

	// read csv file
	f, err := os.Open(*inputFile)
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	// for each row
	for {
		row, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatal(err)
		}
		// validate row
		if len(row) != 4 {
			log.Fatalf("invalid row %v", row)
		}
		// save row
		record := Row{
			Tags:  strings.Split(row[2], ","),
			Notes: row[3],
		}
		record.URL, err = url.Parse(row[0])
		if err != nil {
			log.Errorf("error parsing url %s: %s", row[0], err)
			continue
		}
		if row[1] != "" {
			record.Support, _ = strconv.ParseBool(row[1])
		} else {
			record.Support = true
		}

		err = Save(db, profile.ID, record)
		if err != nil {
			log.Errorf("error saving %s: %s", record.URL, err)
		}
		time.Sleep(100 * time.Millisecond)
	}

}

func Save(db *datastore.Datastore, profileID account.ProfileID, row Row) error {
	ctx := context.Background()
	u := row.URL
	logFields := log.Fields{"profileID": profileID, "legislation_url": u.String()}
	var bill *legislature.Legislation
	matchedURL, err := metadatasites.Lookup(ctx, u)
	if err != nil {
		log.WithContext(ctx).WithFields(logFields).Errorf("metadatasites.Lookup error %#v", err)
	}
	if matchedURL.Hostname() != u.Hostname() {
		log.WithContext(ctx).WithFields(logFields).Infof("metadatasites found URL %q", matchedURL)
	}
	bill, err = resolvers.Lookup(ctx, matchedURL)
	if err != nil {
		return err
	}
	if bill == nil {
		return fmt.Errorf("legislation matching url %q not found", u)
	}
	body := resolvers.Bodies[bill.Body]

	// Save refreshes a bill as well
	var staleSameAs bool
	staleSameAs, err = db.SaveBill(ctx, *bill)
	if err != nil {
		return err
	}
	if staleSameAs {
		// refresh the sameAs bill (if needed)
		sameBill, err := resolvers.Resolvers.Find(body.Bicameral).Refresh(ctx, bill.SameAs)
		if err != nil {
			return err
		}
		_, err = db.SaveBill(ctx, *sameBill)
		if err != nil {
			return err
		}
	}

	bookmark, err := db.GetBookmark(ctx, profileID, account.BookmarkKey(bill.Body, bill.ID))
	if err != nil {
		return err
	}
	if bookmark != nil {
		bookmark.Notes = row.Notes
		bookmark.Tags = row.Tags
		bookmark.Oppose = !row.Support

		// update
		return db.UpdateBookmark(ctx, profileID, *bookmark)
	}

	if bill.SameAs != "" {
		bookmark, err = db.GetBookmark(ctx, profileID, account.BookmarkKey(body.Bicameral, bill.SameAs))
		if err != nil {
			return err
		}
		if bookmark != nil {
			bookmark.Notes = row.Notes
			bookmark.Tags = row.Tags
			bookmark.Oppose = !row.Support

			// update
			return db.UpdateBookmark(ctx, profileID, *bookmark)
		}
	}

	bookmark = &account.Bookmark{
		BodyID:        bill.Body,
		LegislationID: bill.ID,
		Oppose:        !row.Support,
		Created:       time.Now().UTC(),
		Notes:         row.Notes,
		Tags:          row.Tags,

		Legislation: bill,
		Body:        &body,
	}

	err = db.SaveBookmark(ctx, profileID, *bookmark)
	if err != nil && !datastore.IsAlreadyExists(err) {
		return err
	}
	return nil

}
