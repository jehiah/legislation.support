package nysenate

import (
	"context"
	"strconv"

	"github.com/jehiah/legislation.support/internal/legislature"
)

func (a NYSenate) Scorecard(ctx context.Context, bookmarks []legislature.Scorable) (*legislature.Scorecard, error) {
	people, err := a.api.GetMembers(ctx, strconv.Itoa(Sessions.Current().StartYear), "senate")
	if err != nil {
		return nil, err
	}
	s := &legislature.Scorecard{
		Metadata: legislature.ScorecardMetadata{
			PersonTitle: "Senator",
		},
	}
	for _, p := range people {
		s.People = append(s.People, legislature.ScorecardPerson{
			FullName: p.FullName,
			District: strconv.Itoa(p.District),
			// URL:      "https://intro.nyc/councilmembers/" + p.Slug,
			// TODO: Party
		})
	}
	return s, nil
}

func (a NYAssembly) Scorecard(ctx context.Context, bookmarks []legislature.Scorable) (*legislature.Scorecard, error) {
	people, err := a.api.GetMembers(ctx, strconv.Itoa(Sessions.Current().StartYear), "assembly")
	if err != nil {
		return nil, err
	}
	s := &legislature.Scorecard{
		Metadata: legislature.ScorecardMetadata{
			PersonTitle: "Assembly Member",
		},
	}
	for _, p := range people {
		s.People = append(s.People, legislature.ScorecardPerson{
			FullName: p.FullName,
			District: strconv.Itoa(p.District),
			// URL:      "https://intro.nyc/councilmembers/" + p.Slug,
			// TODO: Party
		})
	}
	return s, nil
}
