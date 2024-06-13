package main

import (
	"context"
	"flag"
	"time"

	"cloud.google.com/go/firestore"
	"github.com/jehiah/legislation.support/internal/datastore"
	"github.com/jehiah/legislation.support/internal/legislature"
	log "github.com/sirupsen/logrus"
	"google.golang.org/api/iterator"
)

// tsFmt is used to match logrus timestamp format
// w/ our stdlib log fmt (Ldate | Ltime)
const tsFmt = "2006/01/02 15:04:05"

type Record struct {
	ID   legislature.LegislationID
	Body legislature.BodyID
}

func main() {
	// limit := flag.Int("limit", 500, "limit")
	dryRun := flag.Bool("dry-run", false, "dry run")
	flag.Parse()
	log.SetFormatter(&log.TextFormatter{TimestampFormat: tsFmt, FullTimestamp: true})
	ctx := context.Background()

	// iterate changes and delete those thhat are after epoc
	epoch := time.Date(2024, 6, 11, 0, 0, 0, 0, time.UTC)

	db := datastore.NewClient(ctx)
	iter := db.CollectionGroup("changes").Documents(ctx)
	defer iter.Stop()
	for {
		doc, err := iter.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalf("error fetching changes: %s", err)
		}
		var changes legislature.Changes
		err = doc.DataTo(&changes)
		if err != nil {
			log.Fatalf("error fetching changes: %s", err)
		}
		remove := []any{}
		for _, c := range changes.Sponsors {
			if c.Date.Before(epoch) {
				// log.Printf("skip %s %#v", doc.Ref.Path, c)
				continue
			}
			remove = append(remove, c)
		}
		if len(remove) == 0 {
			continue
		}

		if *dryRun {
			log.Printf("delete %d changes  %s %#v", len(remove), doc.Ref.Path, remove)
		} else {
			log.Printf("delete %d changes  %s %#v", len(remove), doc.Ref.Path, remove)
			_, err := doc.Ref.Update(ctx, []firestore.Update{
				{Path: "Sponsors", Value: firestore.ArrayRemove(remove...)},
			})
			if err != nil {
				log.Fatal(err)
			}
		}
	}

}
