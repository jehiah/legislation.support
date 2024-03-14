package nyc

import (
	"context"
	"strconv"
	"strings"

	"github.com/jehiah/legislation.support/internal/legislature"
	"github.com/jehiah/legislator/db"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

func (a NYC) Scorecard(ctx context.Context, bookmarks []legislature.Scorable) (*legislature.Scorecard, error) {
	s := &legislature.Scorecard{
		Body: &a.body,
		Metadata: legislature.ScorecardMetadata{
			PersonTitle: a.body.MemberName,
		},
		Data: make([]legislature.ScoredBookmark, len(bookmarks)),
	}

	var people []db.Person
	allPeople, err := a.ActivePeople(ctx)
	if err != nil {
		return s, err
	}

	md, err := a.PersonMetadata(ctx)
	if err != nil {
		log.Printf("error: %s", err)
	}
	metaByID := make(map[int]PersonMetadata)
	for _, mm := range md {
		metaByID[mm.ID] = mm
	}

	for _, p := range allPeople {
		switch p.ID {
		case 7780: // skip public advocate
			continue
		}
		var district string
		if md, ok := metaByID[p.ID]; ok {
			district = strconv.Itoa(md.District)
		}
		s.People = append(s.People, legislature.ScorecardPerson{
			FullName: strings.TrimSpace(p.FullName),
			URL:      "https://intro.nyc/councilmembers/" + p.Slug,
			District: district,
			// TODO: Party
		})
		people = append(people, p)
	}

	// TODO: cap concurency
	g := new(errgroup.Group)
	for i, b := range bookmarks {
		i, b := i, b

		g.Go(func() error {
			sb := b.NewScore()
			raw, err := a.Raw(ctx, sb.Legislation)
			if err != nil {
				return err
			}
			sb.Status = raw.StatusName
			sb.Committee = strings.TrimPrefix(raw.BodyName, "Committee on ")
			scores := make(map[string]string)
			for _, sponsor := range raw.Sponsors {
				scores[strings.TrimSpace(sponsor.FullName)] = "Sponsor"
			}
			for _, h := range raw.History {
				for _, v := range h.Votes {
					scores[strings.TrimSpace(v.FullName)] = v.Vote
				}
			}

			for _, p := range s.People {
				sb.Scores = append(sb.Scores, legislature.Score{Status: scores[p.FullName], Desired: !sb.Oppose})
			}
			s.Data[i] = sb
			return nil
		})
	}
	return s, g.Wait()
}
