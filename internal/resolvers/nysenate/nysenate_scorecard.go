package nysenate

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/jehiah/legislation.support/internal/legislature"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

func (a NYSenate) Scorecard(ctx context.Context, bookmarks []legislature.Scorable) (*legislature.Scorecard, error) {
	return a.api.Scorecard(ctx, a.body, bookmarks)
}
func (a NYAssembly) Scorecard(ctx context.Context, bookmarks []legislature.Scorable) (*legislature.Scorecard, error) {
	return a.api.Scorecard(ctx, a.body, bookmarks)
}

func (a NYSenateAPI) Scorecard(ctx context.Context, body legislature.Body, bookmarks []legislature.Scorable) (*legislature.Scorecard, error) {
	s := &legislature.Scorecard{
		Data: make([]legislature.ScoredBookmark, len(bookmarks)),
	}

	var chamber string
	switch body.ID {
	case "nysenate", "ny-senate":
		chamber = "senate"
		s.Metadata.PersonTitle = "Senator"
	case "ny-assembly":
		chamber = "assembly"
		s.Metadata.PersonTitle = "Assembly Member"
	default:
		return nil, fmt.Errorf("invalid chamber %q", chamber)
	}

	people, err := a.GetMembers(ctx, strconv.Itoa(Sessions.Current().StartYear), chamber)
	if err != nil {
		return nil, err
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
	noSameAs := make([]bool, len(bookmarks))
	for i, b := range bookmarks {
		i, b := i, b

		g.Go(func() error {
			sb := b.NewScore()

			// if it's not the same body, get "same as" or skip
			billSession, basePrintNo, _ := strings.Cut(string(sb.Legislation.ID), "-")
			if sb.Legislation.Body != body.ID {
				// lookup same as
				same, err := a.GetBill(ctx, billSession, basePrintNo)
				if err != nil {
					return err
				}
				if same == nil {
					log.Printf("not found %q", sb.Legislation.ID)
				}
				if sameAs := same.GetSameAs(); sameAs != "" {
					log.Printf("[%d] substituting sameAs %s for %s", i, sameAs, sb.Legislation.ID)
					basePrintNo = sameAs
					sb.Legislation = same.Legislation(body.ID)
				} else {
					noSameAs[i] = true
					return nil
				}
			}

			orginalBill, err := a.GetBill(ctx, billSession, basePrintNo)
			votedBill := orginalBill
			if err != nil {
				return err
			}

			// Bills get substituted by bills in the other chamber; in that case votes in both chambers show on the substituted bill
			if orginalBill.SubstitutedBy.BasePrintNo != "" {
				votedBill, err = a.GetBill(ctx, strconv.Itoa(orginalBill.SubstitutedBy.Session), orginalBill.SubstitutedBy.BasePrintNo)
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
	err = g.Wait()
	if err != nil {
		return nil, err
	}
	// delete the items missed from bicameral lookup
	newData := make([]legislature.ScoredBookmark, 0, len(bookmarks))
	seen := make(map[legislature.LegislationID]bool)
	for i, d := range s.Data {
		d := d
		if noSameAs[i] {
			log.Printf("skipping (no same as) %#v", d)
			continue
		}
		if d.Legislation == nil {
			log.Printf("d[%d].Legislation == nil", i)
			continue
		}
		if seen[d.Legislation.ID] {
			// TODO: skip the one from the other chamber
			log.Printf("duplicated in profile (listed for both chambers) %s", d.Legislation.ID)
			continue
		}
		seen[d.Legislation.ID] = true
		newData = append(newData, d)
	}
	// TODO re-sort

	s.Data = newData

	return s, nil
}
