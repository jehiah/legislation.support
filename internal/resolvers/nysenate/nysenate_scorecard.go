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

func splitLegislationID(l legislature.LegislationID) (string, string) {
	session, basePrint, _ := strings.Cut(string(l), "-")
	return session, basePrint
}

func (a NYSenateAPI) Scorecard(ctx context.Context, body legislature.Body, bookmarks []legislature.Scorable) (*legislature.Scorecard, error) {
	s := &legislature.Scorecard{
		Data: make([]legislature.ScoredBookmark, len(bookmarks)),
	}

	var c chamber
	switch body.ID {
	case "nysenate", "ny-senate":
		c = senateChamber
		s.Metadata.PersonTitle = "Senator"
	case "ny-assembly":
		c = assemblyChamber
		s.Metadata.PersonTitle = "Assembly Member"
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

			// if it's not the same body, get "same as" or skip
			billSession, basePrintNo, _ := strings.Cut(string(sb.Legislation.ID), "-")
			orginalBill, err := a.GetBill(ctx, billSession, basePrintNo)
			if err != nil {
				return err
			}
			if orginalBill == nil {
				log.Printf("not found %q", sb.Legislation.ID)
				return nil
			}
			if sb.Legislation.Body != body.ID {
				// lookup same as; if exists in the same body, use that
				if sameAs := orginalBill.GetSameAs(); sameAs != "" {
					log.Printf("[%d] substituting %s (%s) sameAs => %s", i, sb.Legislation.ID, sb.Legislation.DisplayID, sameAs)
					sameAsSession, sameAsBasePrintNo := splitLegislationID(sameAs)
					orginalBill, err = a.GetBill(ctx, sameAsSession, sameAsBasePrintNo)
					if err != nil {
						return err
					}
					// reset Legislation after "sameAs" lookup
					sb.Legislation = orginalBill.Legislation(body.ID)
				} else {
					noSameAs[i] = true
					return nil
				}
			}

			if sameAs := orginalBill.GetSameAs(); sameAs != "" {
				_, sameAsBasePrintNo := splitLegislationID(sameAs)
				sb.Legislation.DisplayID += " / " + sameAsBasePrintNo
			}
			votedBill := orginalBill

			// Bills get substituted by bills in the other chamber; in that case votes in both chambers show on the substituted bill
			if orginalBill.SubstitutedBy.BasePrintNo != "" {
				log.Printf("substitution %s-%s SubstitutedBy => %d-%s", billSession, basePrintNo, orginalBill.SubstitutedBy.Session, orginalBill.SubstitutedBy.BasePrintNo)
				votedBill, err = a.GetBill(ctx, strconv.Itoa(orginalBill.SubstitutedBy.Session), orginalBill.SubstitutedBy.BasePrintNo)
				if err != nil {
					return err
				}
			}

			// FIXME: https://github.com/nysenate/OpenLegislation/issues/122
			if c == assemblyChamber && orginalBill.BillType.Chamber == "ASSEMBLY" {
				extraVotes, err := a.AssemblyVotes(ctx, people, strconv.Itoa(orginalBill.Session), orginalBill.BasePrintNo)
				if err != nil {
					return err
				}
				votedBill.Votes.Items = append(votedBill.Votes.Items, extraVotes.Votes.Items...)
			}

			sb.Status = votedBill.Status.StatusDesc
			sb.Committee = orginalBill.Status.CommitteeName
			scores := make(map[int]string)
			remaining := make(map[int]bool)
			for _, sponsor := range orginalBill.GetSponsors() {
				scores[sponsor.MemberID] = "Sponsor"
				remaining[sponsor.MemberID] = true
			}

			for _, v := range votedBill.GetVotes().Filter(orginalBill.BillType.Chamber) {
				if v.MemberID == 0 {
					log.WithField("session", billSession).WithField("bill", basePrintNo).Warnf("unexpected memberID=0 %#v", v)
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
