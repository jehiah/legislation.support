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
	"github.com/jehiah/legislation.support/internal/resolvers/nyc"
	log "github.com/sirupsen/logrus"
)

// tsFmt is used to match logrus timestamp format
// w/ our stdlib log fmt (Ldate | Ltime)
const tsFmt = "2006/01/02 15:04:05"

type SameAs struct {
	SameAsPrintNo    string   `json:",omitempty"`
	PreviousVersions []string `json:",omitempty"`
}

func flipNyPrintNo(s string) legislature.LegislationID {
	a, b, _ := strings.Cut(s, "-")
	return legislature.LegislationID(b + "-" + a)
}

func supportsResubmit(bodyID legislature.BodyID) bool {
	switch bodyID {
	case resolvers.NYSenate.ID:
	case resolvers.NYAssembly.ID:
	case resolvers.NYCCouncil.ID:
	default:
		return false
	}
	return true
}

func buildNYResubmitMapping(nyLegislationPath string) legislature.ResubmitMapping {
	mapping := make(legislature.ResubmitMapping)
	files, err := filepath.Glob(filepath.Join(nyLegislationPath, "bills", "2025-index.json"))
	if err != nil {
		log.Fatal(err)
	}
	for _, fn := range files {
		var index map[string]SameAs
		body, err := os.ReadFile(fn)
		if err != nil {
			log.Fatal(err)
		}
		if err := json.Unmarshal(body, &index); err != nil {
			log.Fatal(err)
		}
		for printNo, sameAs := range index {
			for _, prevPrintNo := range sameAs.PreviousVersions {
				prev := legislature.GlobalID{
					BodyID:        resolvers.NYSenate.ID,
					LegislationID: flipNyPrintNo(prevPrintNo),
				}
				current := legislature.GlobalID{
					BodyID:        resolvers.NYSenate.ID,
					LegislationID: flipNyPrintNo(printNo),
				}
				if strings.HasPrefix(prevPrintNo, "A") {
					prev.BodyID = resolvers.NYAssembly.ID
				}
				if strings.HasPrefix(printNo, "A") {
					current.BodyID = resolvers.NYAssembly.ID
				}
				mapping[prev] = current
			}
		}
	}
	return mapping
}
func buildNYCReesubmitMapping(ctx context.Context) legislature.ResubmitMapping {
	m := make(legislature.ResubmitMapping)
	nycAPI := nyc.New(resolvers.NYCCouncil)
	currentYear := time.Now().Year()
	for year := 2020; year <= currentYear; year++ {
		v, err := nycAPI.Resubmit(context.Background(), year)
		if err != nil {
			log.Fatal(err)
		}
		m.Extend(v)
	}
	return m
}

func main() {
	nyLegislationPath := flag.String("ny-legislation-path", "../../../ny_legislation", "path to ny-legislation repo")
	profileIDStr := flag.String("profile-id", "jehiah-nyc", "profile id")
	dryRun := flag.Bool("dry-run", false, "dry run")
	flag.Parse()
	log.SetFormatter(&log.TextFormatter{TimestampFormat: tsFmt, FullTimestamp: true})
	ctx := context.Background()
	db := datastore.New(datastore.NewClient(ctx))

	mapping := buildNYResubmitMapping(*nyLegislationPath)
	mapping.Extend(buildNYCReesubmitMapping(ctx))

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
		if bb.Legislation.Session.Active() || !supportsResubmit(bb.BodyID) {
			continue
		}
		records = append(records, bb)
	}

	if len(records) == 0 {
		log.Info("no matching archived bookmarks")
		return
	}
	log.WithField("profile", profileID).Infof("checking %d archived bookmarks for resubmit", len(records))

	for _, record := range records {

		newMaping, ok := mapping[legislature.GlobalID{
			BodyID:        record.BodyID,
			LegislationID: record.LegislationID,
		}]
		logFields := log.Fields{
			"bodyID":        record.BodyID,
			"legislationID": record.LegislationID,
			"newMapping":    newMaping,
		}
		if !ok {
			log.Printf("no mapping for %s %s", record.BodyID, record.LegislationID)
			continue
		}
		log.Printf("mapping %s %s => %s", record.BodyID, record.LegislationID, newMaping)

		bookmark, err := db.GetBookmark(ctx, profile.ID, account.BookmarkKey(newMaping.BodyID, newMaping.LegislationID))
		if err != nil {
			log.WithFields(logFields).Fatalf("%+v", err)
		}
		if bookmark != nil {
			log.Infof("bookmark already exists for %s => %s", record.LegislationID, newMaping.LegislationID)
			continue
		}

		bill, err := resolvers.Resolvers.Find(newMaping.BodyID).Refresh(ctx, newMaping.LegislationID)
		if err != nil {
			log.WithFields(logFields).Fatal(err)
		}
		if bill == nil {
			log.WithFields(logFields).Fatalf("legislation %s %s not found", newMaping.BodyID, newMaping.LegislationID)
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

		if bill.SameAs != "" {
			bookmark, err := db.GetBookmark(ctx, profile.ID, account.BookmarkKey(body.Bicameral, bill.SameAs))
			if err != nil {
				log.Fatal(err)
			}
			if bookmark != nil {
				log.Infof("bookmark already exists for %s => SameAs %s (also %s)", record.LegislationID, bill.SameAs, bill.ID)
				continue
			}
		}

		if *dryRun {
			log.Warnf("would resubmit %s as %s", record.LegislationID, newMaping)
			continue
		}

		log.WithField("id", record.LegislationID).Warnf("resubmiting %s as %s", record.LegislationID, newMaping)

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
