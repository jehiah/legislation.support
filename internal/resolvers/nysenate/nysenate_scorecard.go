package nysenate

import (
	"context"
	"strconv"
	"strings"

	"github.com/jehiah/legislation.support/internal/legislature"
	"golang.org/x/sync/errgroup"
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
		Data: make([]legislature.ScoredBookmark, len(bookmarks)),
	}
	for _, p := range people {
		s.People = append(s.People, legislature.ScorecardPerson{
			FullName: p.FullName,
			District: strconv.Itoa(p.District),
			// URL:      "https://intro.nyc/councilmembers/" + p.Slug,
			// TODO: Party
		})
	}

	// convert to bicameral and de-dupe
	// var finalBookmarks []legislature.Scorable

	g := new(errgroup.Group)
	for i, b := range bookmarks {
		i, b := i, b

		g.Go(func() error {
			sb := b.NewScore()
			billSession, basePrintNo, _ := strings.Cut(string(sb.Legislation.ID), "-")
			orginalBill, err := a.api.GetBill(ctx, billSession, basePrintNo)
			votedBill := orginalBill
			if err != nil {
				return err
			}

			if orginalBill.SubstitutedBy.BasePrintNo != "" {
				votedBill, err = a.api.GetBill(ctx, strconv.Itoa(orginalBill.SubstitutedBy.Session), orginalBill.SubstitutedBy.BasePrintNo)
				if err != nil {
					return err
				}
			}

			sb.Status = votedBill.Status.StatusDesc
			sb.Committee = orginalBill.Status.CommitteeName
			scores := make(map[int]string)
			for _, sponsor := range orginalBill.GetSponsors() {
				scores[sponsor.MemberID] = "Sponsor"
			}

			for _, v := range votedBill.GetVotes().Filter(orginalBill.BillType.Chamber) {
				scores[v.MemberID] = v.Vote
			}

			for _, p := range people {
				sb.Scores = append(sb.Scores, legislature.Score{Status: scores[p.MemberID], Desired: !sb.Oppose})
			}
			s.Data[i] = sb
			return nil
		})
	}
	return s, g.Wait()
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
