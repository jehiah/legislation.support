package nysenate

import (
	"context"
	"fmt"
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

func splitLegislationID(l legislature.LegislationID) (string, string) {
	session, basePrint, _ := strings.Cut(string(l), "-")
	return session, basePrint
}

func (a NYSenateAPI) Scorecard(ctx context.Context, body legislature.Body, bookmarks []legislature.Scorable) (*legislature.Scorecard, error) {
	s := &legislature.Scorecard{
		Body: &body,
		Metadata: legislature.ScorecardMetadata{
			PersonTitle: body.MemberName,
		},
		Data: make([]legislature.ScoredBookmark, len(bookmarks)),
	}

	var c chamber
	switch body.ID {
	case "nysenate", "ny-senate":
		c = senateChamber
	case "ny-assembly":
		c = assemblyChamber
	default:
		return nil, fmt.Errorf("invalid chamber %s", body.ID)
	}

	people, err := a.GetMembers(ctx, Sessions.Current(), c)
	if err != nil {
		return nil, err
	}
	seenPeople := make(map[int]bool)
	for _, p := range people {
		if seenPeople[p.NumericID] {
			continue
		}
		seenPeople[p.NumericID] = true
		s.People = append(s.People, legislature.ScorecardPerson{
			FullName: p.FullName,
			District: p.District,
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

			bill := sb.Legislation.ID
			otherBill := sb.Legislation.SameAs
			if sb.Legislation.Body != body.ID {
				bill, otherBill = otherBill, bill
			}
			if bill == "" {
				// no same-as
				noSameAs[i] = true
				s.Data[i] = sb
				return nil
			}

			// if it's not the same body, get "same as" or skip
			billSession, basePrintNo := splitLegislationID(bill)
			billData, err := a.GetBill(ctx, billSession, basePrintNo)
			if err != nil {
				return err
			}
			sb.Status = billData.Status.StatusDesc
			sb.Committee = billData.Status.CommitteeName

			var otherBillData *Bill
			if otherBill != "" {
				sb.Legislation = billData.Legislation(body.ID)
				otherBillSession, otherBasePrintNo := splitLegislationID(otherBill)
				otherBillData, err = a.GetBill(ctx, otherBillSession, otherBasePrintNo)
				if err != nil {
					return err
				}

				if billData.SubstitutedBy.BasePrintNo == otherBasePrintNo {
					sb.Status = otherBillData.Status.StatusDesc
					sb.Committee = otherBillData.Status.CommitteeName
				}
			}

			// Assembly votes are not exposed in the NY Senate API
			// https://github.com/nysenate/OpenLegislation/issues/122
			if c == assemblyChamber {
				extraVotes, err := a.AssemblyVotes(ctx, people, billSession, basePrintNo)
				if err != nil {
					return err
				}
				for _, vi := range extraVotes.Votes.Items {
					if vi.VoteType == "Held for Consideration" {
						// https://nyassembly.gov/leg/?default_fld=&leg_video=&bn=A06141&term=&Summary=Y&Actions=Y&Committee%26nbspVotes=Y&Floor%26nbspVotes=Y&Text=Y
						// Some bills are "Held for Consideration" which doesn't represent a vote for the bill - it's a vote to hold a bill
						continue
					}
					billData.Votes.Items = append(billData.Votes.Items, vi)
				}
			}

			scores := make(map[int]string)
			remaining := make(map[int]bool)
			for _, sponsor := range billData.GetSponsors() {
				scores[sponsor.MemberID] = "Sponsor"
				remaining[sponsor.MemberID] = true
			}

			for _, v := range billData.GetVotes().Filter(billData.BillType.Chamber) {
				if v.MemberID == 0 {
					log.WithField("session", billSession).WithField("bill", basePrintNo).Warnf("unexpected memberID=0 %#v", v)
					continue
				}
				scores[v.MemberID] = v.Vote
				remaining[v.MemberID] = true
			}

			seenPeople := make(map[int]bool)
			for _, p := range people {
				if seenPeople[p.NumericID] {
					continue
				}
				seenPeople[p.NumericID] = true
				delete(remaining, p.NumericID)
				sb.Scores = append(sb.Scores, legislature.Score{Status: scores[p.NumericID], Desired: !sb.Oppose})
			}
			for id := range remaining {
				if id != 0 {
					log.WithField("session", billSession).WithField("bill", basePrintNo).Warnf("member id %d has vote %v", id, scores[id])
				}
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
			// log.Printf("skipping (no same as) %s", d.Legislation.ID)
			continue
		}
		if d.Legislation == nil {
			log.Printf("d[%d].Legislation == nil", i)
			continue
		}
		if seen[d.Legislation.ID] {
			// TODO: skip if both bills were listed
			log.Printf("duplicated in profile (listed for both chambers) %s", d.Legislation.ID)
			continue
		}
		seen[d.Legislation.ID] = true
		seen[d.Legislation.SameAs] = true
		newData = append(newData, d)
	}
	// TODO re-sort

	s.Data = newData

	return s, nil
}
