package nyc

import (
	"context"
	"strconv"
	"strings"

	"github.com/jehiah/legislation.support/internal/legislature"
	"github.com/jehiah/legislator/db"
	log "github.com/sirupsen/logrus"
)

func (a NYC) Members(ctx context.Context, session legislature.Session) ([]legislature.Member, error) {

	var allPeople []db.Person
	var err error
	if session.Active() {
		allPeople, err = a.ActivePeople(ctx)
	} else {
		allPeople, err = a.AllPeople(ctx)
	}

	md, err := a.PersonMetadata(ctx)
	if err != nil {
		log.Printf("error: %s", err)
		return nil, err
	}
	metaByID := make(map[int]PersonMetadata)
	for _, mm := range md {
		metaByID[mm.ID] = mm
	}

	var people []legislature.Member
	for _, p := range allPeople {
		switch p.ID {
		case 7780: // skip public advocate
			continue
		}
		var isActive bool
		for _, office := range p.OfficeRecords {
			if office.BodyID != 1 {
				continue
			}
			if session.Overlaps(office.Start, office.End) {
				isActive = true
			}
		}

		var district string
		if md, ok := metaByID[p.ID]; ok {
			district = strconv.Itoa(md.District)
		}
		if isActive {
			people = append(people, legislature.Member{
				NumericID: p.ID,
				FullName:  strings.TrimSpace(p.FullName),
				URL:       "https://intro.nyc/councilmembers/" + p.Slug,
				Slug:      p.Slug,
				District:  district,
				// TODO: Party
			})
		}
	}
	return people, nil

}
