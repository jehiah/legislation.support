package nyc

import (
	"context"
	"strings"

	"github.com/jehiah/legislation.support/internal/legislature"
	"github.com/jehiah/legislator/db"
)

func (a NYC) Scorecard(ctx context.Context, bookmarks []legislature.Scorable) (*legislature.Scorecard, error) {
	s := &legislature.Scorecard{
		Metadata: legislature.ScorecardMetadata{
			PersonTitle: "Council Member",
		},
	}

	var people []db.Person
	allPeople, err := a.ActivePeople(ctx)
	if err != nil {
		return s, err
	}

	for _, p := range allPeople {
		switch p.ID {
		case 7780: // skip public advocate
			continue
		}
		s.People = append(s.People, legislature.ScorecardPerson{
			FullName: strings.TrimSpace(p.FullName),
			URL:      "https://intro.nyc/councilmembers/" + p.Slug,
			// TODO: Party, District
		})
		people = append(people, p)
	}

	// TODO: lookup in a goroutine
	for _, b := range bookmarks {
		sb := b.NewScore()

		raw, err := a.Raw(ctx, sb.Legislation)
		if err != nil {
			return s, err
		}
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
		s.Data = append(s.Data, sb)
	}
	return s, nil
}
