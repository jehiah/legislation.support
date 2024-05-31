package main

import (
	"context"
	"flag"
	"time"

	"github.com/jehiah/legislation.support/internal/datastore"
	"github.com/jehiah/legislation.support/internal/legislature"
	"github.com/jehiah/legislation.support/internal/resolvers"
	log "github.com/sirupsen/logrus"
)

// tsFmt is used to match logrus timestamp format
// w/ our stdlib log fmt (Ldate | Ltime)
const tsFmt = "2006/01/02 15:04:05"

type Record struct {
	ID   legislature.LegislationID
	Body legislature.BodyID
}

func main() {
	limit := flag.Int("limit", 500, "limit")
	flag.Parse()
	log.SetFormatter(&log.TextFormatter{TimestampFormat: tsFmt, FullTimestamp: true})
	ctx := context.Background()
	db := datastore.New(datastore.NewClient(ctx))

	// for each bill check if it's SameAs exists
	// if it doesn't fetch and save it

	haveBill := make(map[Record]bool)
	sameAs := make(map[Record]bool)
	// get all bills

	err := db.GetAllBills(ctx, func(bill legislature.Legislation) error {
		if !bill.Session.Active() {
			return nil
		}
		// check if we already have this bill
		haveBill[Record{ID: bill.ID, Body: bill.Body}] = true

		if bill.SameAs != "" {
			otherBody := resolvers.Bodies[bill.Body].Bicameral
			sameAs[Record{ID: bill.SameAs, Body: otherBody}] = true
		}
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	// check if we have all SameAs bills
	var count int
	for record := range sameAs {
		if !haveBill[record] {
			count++
			if count > *limit {
				log.Printf("limit %d reached", *limit)
				break
			}
			log.Printf("missing %s %s", record.Body, record.ID)
			// fetch and save bill
			resolver := resolvers.Resolvers.Find(record.Body)
			bill, err := resolver.Refresh(ctx, record.ID)
			if err != nil {
				log.Fatalf("error fetching %s %s: %s", record.Body, record.ID, err)
			}
			_, err = db.SaveBill(ctx, *bill)
			if err != nil {
				log.Fatalf("error saving %s %s: %s", record.Body, record.ID, err)
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

}
