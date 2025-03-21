package main

import (
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jehiah/legislation.support/internal/account"
	"github.com/jehiah/legislation.support/internal/datastore"
	"github.com/jehiah/legislation.support/internal/legislature"
	"github.com/jehiah/legislation.support/internal/resolvers"
	"github.com/jehiah/legislation.support/internal/resolvers/nysenate"
	log "github.com/sirupsen/logrus"
)

// tsFmt is used to match logrus timestamp format
// w/ our stdlib log fmt (Ldate | Ltime)
const tsFmt = "2006/01/02 15:04:05"

type SameAs struct {
	SameAsPrintNo    legislature.LegislationID   `json:",omitempty"`
	PreviousVersions []legislature.LegislationID `json:",omitempty"`
}

func flip(s legislature.LegislationID) legislature.LegislationID {
	a, b, _ := strings.Cut(string(s), "-")
	return legislature.LegislationID(b + "-" + a)
}

func main() {
	nyLegislationPath := flag.String("ny-legislation-path", "../../../ny_legislation", "path to ny-legislation repo")
	profileIDStr := flag.String("profile-id", "jehiah-nyc", "profile id")
	dryRun := flag.Bool("dry-run", false, "dry run")
	flag.Parse()
	log.SetFormatter(&log.TextFormatter{TimestampFormat: tsFmt, FullTimestamp: true})
	ctx := context.Background()
	db := datastore.New(datastore.NewClient(ctx))

	mapping := make(map[legislature.LegislationID]legislature.LegislationID)

	files, err := filepath.Glob(filepath.Join(*nyLegislationPath, "bills", "2025-index.json"))
	if err != nil {
		log.Fatal(err)
	}
	for _, fn := range files {
		var index map[legislature.LegislationID]SameAs
		body, err := os.ReadFile(fn)
		if err != nil {
			log.Fatal(err)
		}
		if err := json.Unmarshal(body, &index); err != nil {
			log.Fatal(err)
		}
		for printNo, sameAs := range index {
			for _, prevPrintNo := range sameAs.PreviousVersions {
				mapping[flip(prevPrintNo)] = flip(printNo)
			}
		}
	}

	profileID := account.ProfileID(*profileIDStr)
	if !account.IsValidProfileID(profileID) {
		log.Fatalf("invalid profile id %s", profileID)
	}

	profile, err := db.GetProfile(ctx, profileID)
	if err != nil {
		log.Fatal(err)
	}
	if profile == nil {
		log.Fatalf("profile %s not found", profileID)
	}

	// load bookmarks that are archived
	var records []account.Bookmark
	b, err := db.GetProfileBookmarks(ctx, profileID)
	if err != nil {
		log.WithField("profileID", profileID).Fatalf("%s", err)
	}
	for _, bb := range b {
		if !bb.Legislation.Session.Active() {
			switch bb.BodyID {
			case resolvers.NYSenate.ID:
			case resolvers.NYAssembly.ID:
			default:
				continue
			}
			records = append(records, bb)
		}
	}

	if len(records) == 0 {
		log.Info("no matching archived bookmarks")
		return
	}
	log.WithField("profile", profileID).Infof("checking %d archived bookmarks for resubmit", len(records))

	a := nysenate.NewAPI(os.Getenv("NY_SENATE_TOKEN"))

	for _, record := range records {
		switch record.BodyID {
		case resolvers.NYSenate.ID:
		case resolvers.NYAssembly.ID:
		default:
			continue
		}

		printNo, ok := mapping[record.LegislationID]
		if !ok {
			log.Printf("no mapping for %s", record.LegislationID)
			continue
		}

		matchedURL := a.Link(printNo)
		bill, err := resolvers.Lookup(ctx, matchedURL)
		if err != nil {
			log.Fatal(err)
		}
		if bill == nil {
			log.Fatalf("legislation matching url %q not found", matchedURL)
		}
		body := resolvers.Bodies[bill.Body]

		if !*dryRun {
			staleSameAs, err := db.SaveBill(ctx, *bill)
			if err != nil {
				log.Fatal(err)
			}
			if staleSameAs || bill.SameAs != "" {
				// refresh the sameAs bill (if needed)
				sameBill, err := resolvers.Resolvers.Find(body.Bicameral).Refresh(ctx, bill.SameAs)
				if err != nil {
					log.Fatal(err)
				}
				_, err = db.SaveBill(ctx, *sameBill)
				if err != nil {
					log.Fatal(err)
				}
			}
		}

		bookmark, err := db.GetBookmark(ctx, profile.ID, account.BookmarkKey(bill.Body, bill.ID))
		if err != nil {
			log.Fatal(err)
		}
		if bookmark != nil {
			log.Infof("bookmark already exists for %s", bill.ID)
			continue
		}

		if bill.SameAs != "" {
			bookmark, err := db.GetBookmark(ctx, profile.ID, account.BookmarkKey(body.Bicameral, bill.SameAs))
			if err != nil {
				log.Fatal(err)
			}
			if bookmark != nil {
				log.Infof("bookmark already exists for SameAs %s (bill %s)", bill.SameAs, bill.ID)
				continue
			}
		}

		if *dryRun {
			log.Warnf("would resubmit %s as %s", record.LegislationID, printNo)
			continue
		}

		log.WithField("id", record.LegislationID).Warnf("resubmit %s as %s", record.LegislationID, printNo)

		// resubmit
		err = db.UpdateBookmark(ctx, profile.ID, account.Bookmark{
			Created:       time.Now().UTC(),
			LegislationID: bill.ID,
			BodyID:        bill.Body,
			Tags:          record.Tags,
			Notes:         record.Notes,
			Oppose:        record.Oppose,
			Legislation:   bill,
			Body:          &body,
		})
		if err != nil && !datastore.IsAlreadyExists(err) {
			log.Fatal(err)
		}

	}

}
