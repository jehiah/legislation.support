package congress

import (
	"context"
	"strings"

	"github.com/jehiah/legislation.support/internal/legislature"
	"golang.org/x/sync/errgroup"
)

// skip votes like "Motion to reconsider laid on the table"
func (a BillAction) IsVoteIrelevant() bool {
	switch {
	case strings.HasPrefix(a.Text, "Motion to reconsider laid on the table"):
		return true
	}
	return false
}

func scorecardForCongress(ctx context.Context, body legislature.Body, api *CongressAPI, chamber string, items []legislature.Scorable) (*legislature.Scorecard, error) {
	s := &legislature.Scorecard{
		Body: &body,
		Metadata: legislature.ScorecardMetadata{
			PersonTitle: body.MemberName,
		},
		Data: make([]legislature.ScoredBookmark, len(items)),
	}
	var err error
	var members []Member
	if len(items) > 0 {
		congress, _, _, _ := parseBillID(items[0].NewScore().Legislation.ID)
		members, err = api.Members(ctx, SessionForCongress(congress))
		if err != nil {
			return nil, err
		}
	}

	people, peopleIDs, err := congressScorecardPeople(ctx, api, chamber, members)
	if err != nil {
		return nil, err
	}
	s.People = people

	g := new(errgroup.Group)
	g.SetLimit(5)
	for i, item := range items {
		i, item := i, item
		g.Go(func() error {
			sb := item.NewScore()
			congress, billType, number, err := parseBillID(sb.Legislation.ID)
			if err != nil {
				return err
			}

			bill, err := api.GetBill(ctx, congress, billType, number)
			if err != nil {
				return err
			}

			// actions, err = api.GetActions(ctx, congress, billType, number)
			// if err != nil {
			// 	return nil, err
			// }

			sb.Status = bill.LatestAction.Text
			// sb.Committee = committeeFromActions(actions)

			scores := make(map[string]string)
			for _, sponsor := range bill.Sponsors {
				if sponsor.BioguideID == "" {
					continue
				}
				scores[sponsor.BioguideID] = "Sponsor"
			}
			for _, cosponsor := range bill.Cosponsors.Items {
				if cosponsor.BioguideID == "" {
					continue
				}
				scores[cosponsor.BioguideID] = "Sponsor"
			}

			// for _, action := range actions {
			// 	if action.IsVoteIrelevant() {
			// 		continue
			// 	}
			// 	for _, rv := range action.RecordedVotes {
			// 		if !voteMatchesChamber(rv.Chamber, chamber) {
			// 			continue
			// 		}
			// 		vote, err := api.GetVoteXML(ctx, rv.URL)
			// 		if err != nil {
			// 			return err
			// 		}
			// 		for _, member := range vote.Members {
			// 			memberID := member.LISMemberID
			// 			if memberID == "" {
			// 				continue
			// 			}
			// 			scores[memberID] = normalizeVoteCast(member.VoteCast)
			// 		}
			// 	}
			// }

			for idx := range s.People {
				status := scores[peopleIDs[idx]]
				sb.Scores = append(sb.Scores, legislature.Score{Status: status, Desired: !sb.Oppose})
			}

			s.Data[i] = sb
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return s, nil
}

func congressScorecardPeople(ctx context.Context, api *CongressAPI, chamber string, members []Member) ([]legislature.ScorecardPerson, []string, error) {

	var people []legislature.ScorecardPerson
	var ids []string
	for _, m := range members {
		isHouseMember := m.District.String() != ""
		if chamber == "House" && !isHouseMember {
			continue
		}
		if chamber == "Senate" && isHouseMember {
			continue
		}
		_, shortName := normalizeCongressName(m.Name)
		person := legislature.ScorecardPerson{
			FullName: shortName,
			Party:    normalizeParty(m.PartyName),
			District: normalizeDistrict(m.District, m.State),
			URL:      m.URL,
		}
		ids = append(ids, m.BioguideID)
		people = append(people, person)
	}

	return people, ids, nil
}

func committeeFromActions(actions []BillAction) string {
	for _, action := range actions {
		for _, committee := range action.Committees {
			if committee.Name != "" {
				return committee.Name
			}
		}
	}
	return ""
}

func voteMatchesChamber(voteChamber, chamber string) bool {
	if chamber == "" || voteChamber == "" {
		return true
	}
	return voteChamber == chamber
}

func normalizeVoteCast(vote string) string {
	switch strings.ToLower(strings.TrimSpace(vote)) {
	case "yea", "yes", "aye":
		return "Aye"
	case "nay", "no":
		return "Nay"
	case "present":
		return "Present"
	case "not voting", "absent", "not-voting":
		return "Not Voting"
	default:
		return strings.TrimSpace(vote)
	}
}
