package congress

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/jehiah/legislation.support/internal/legislature"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"
)

// SupportedDomains for House
func (h House) SupportedDomains() []string {
	return []string{"www.congress.gov", "congress.gov"}
}

// SupportedDomains for Senate
func (s Senate) SupportedDomains() []string {
	return []string{"www.congress.gov", "congress.gov"}
}

// Lookup finds a House bill from a URL
func (h House) Lookup(ctx context.Context, u *url.URL) (*legislature.Legislation, error) {
	bill, err := h.api.Lookup(ctx, u)
	if err != nil {
		return nil, err
	}
	if bill.OriginChamber != "House" {
		return nil, fmt.Errorf("bill %s has OriginChamber:%q", bill.Number, bill.OriginChamber)
	}
	return bill.ToLegislation(h.body.ID)
}

// Lookup finds a Senate bill from a URL
func (s Senate) Lookup(ctx context.Context, u *url.URL) (*legislature.Legislation, error) {
	bill, err := s.api.Lookup(ctx, u)
	if err != nil {
		return nil, err
	}
	if bill.OriginChamber != "Senate" {
		return nil, fmt.Errorf("bill %s has OriginChamber:%q", bill.Number, bill.OriginChamber)
	}
	return bill.ToLegislation(s.body.ID)
}

// billTypeNameToCode converts URL bill type name to API code
func billTypeNameToCode(name string) string {
	switch name {
	case "house-bill":
		return "hr"
	case "senate-bill":
		return "s"
	case "house-joint-resolution":
		return "hjres"
	case "senate-joint-resolution":
		return "sjres"
	case "house-concurrent-resolution":
		return "hconres"
	case "senate-concurrent-resolution":
		return "sconres"
	case "house-resolution":
		return "hres"
	case "senate-resolution":
		return "sres"
	default:
		return ""
	}
}

// Refresh fetches updated data for a House bill
func (h House) Refresh(ctx context.Context, billID legislature.LegislationID) (*legislature.Legislation, error) {
	congress, billType, number, err := parseBillID(billID)
	if err != nil {
		return nil, err
	}

	// Validate it's a House bill
	if !strings.HasPrefix(billType, "h") {
		return nil, fmt.Errorf("House resolver cannot refresh Senate bill: %q", billID)
	}

	bill, err := h.api.GetBill(ctx, congress, billType, number)
	if err != nil {
		return nil, err
	}

	return bill.ToLegislation(h.body.ID)
}

// Refresh fetches updated data for a Senate bill
func (s Senate) Refresh(ctx context.Context, billID legislature.LegislationID) (*legislature.Legislation, error) {
	congress, billType, number, err := parseBillID(billID)
	if err != nil {
		return nil, err
	}

	// Validate it's a Senate bill
	if !strings.HasPrefix(billType, "s") {
		return nil, fmt.Errorf("Senate resolver cannot refresh House bill: %q", billID)
	}

	bill, err := s.api.GetBill(ctx, congress, billType, number)
	if err != nil {
		return nil, err
	}

	return bill.ToLegislation(s.body.ID)
}

func Link(l legislature.LegislationID) *url.URL {
	congress, billType, number, err := parseBillID(l)
	if err != nil {
		log.Errorf("congress.Link: unable to parse bill ID %q: %v", l, err)
	}
	return &url.URL{
		Scheme: "https",
		Host:   "www.congress.gov",
		Path:   fmt.Sprintf("/bill/%dth-congress/%s/%s", congress, billTypeToName(billType), number),
	}
}

// Link generates a URL for a House bill
func (h House) Link(l legislature.LegislationID) *url.URL {
	return Link(l)
}

// Link generates a URL for a Senate bill
func (s Senate) Link(l legislature.LegislationID) *url.URL {
	return Link(l)
}

// DisplayID returns a formatted display ID (e.g., "H.R. 1234")
func (h House) DisplayID(l legislature.LegislationID) string {
	_, billType, number, err := parseBillID(l)
	if err != nil {
		return string(l)
	}
	return formatDisplayID(billType, number)
}

// DisplayID returns a formatted display ID (e.g., "S. 874")
func (s Senate) DisplayID(l legislature.LegislationID) string {
	_, billType, number, err := parseBillID(l)
	if err != nil {
		return string(l)
	}
	return formatDisplayID(billType, number)
}

// Scorecard is not yet implemented for Congress
func (h House) Scorecard(ctx context.Context, items []legislature.Scorable) (*legislature.Scorecard, error) {
	return scorecardForCongress(ctx, h.body, h.api, "House", items)
}

// Scorecard is not yet implemented for Congress
func (s Senate) Scorecard(ctx context.Context, items []legislature.Scorable) (*legislature.Scorecard, error) {
	return scorecardForCongress(ctx, s.body, s.api, "Senate", items)
}

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

	people, peopleIDs, err := congressScorecardPeople(ctx, api, chamber)
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

			// actions, err := api.GetActions(ctx, congress, billType, number)
			// if err != nil {
			// 	return err
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

func congressScorecardPeople(ctx context.Context, api *CongressAPI, chamber string) ([]legislature.ScorecardPerson, []string, error) {
	members, err := api.Members(ctx, Sessions.Current())
	if err != nil {
		return nil, nil, err
	}

	var people []legislature.ScorecardPerson
	var ids []string
	for _, m := range members {
		isHouseMember := normalizeDistrict(m.District) != ""
		if chamber == "House" && !isHouseMember {
			continue
		}
		if chamber == "Senate" && isHouseMember {
			continue
		}
		_, shortName := normalizeCongressName(m.Name)
		district := normalizeDistrict(m.District)
		if district != "" && m.State != "" {
			district = fmt.Sprintf("%s-%s", m.State, district)
		} else if district == "" {
			district = m.State
		}
		person := legislature.ScorecardPerson{
			FullName: shortName,
			Party:    m.PartyName,
			District: district,
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
	return strings.EqualFold(voteChamber, chamber)
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
