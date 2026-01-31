package congress

import (
	"context"
	"encoding/xml"
	"net/http"
)

// HouseRollCallVote represents the XML structure for House votes from clerk.house.gov
// Example: https://clerk.house.gov/evs/2025/roll190.xml
type HouseRollCallVote struct {
	XMLName      xml.Name          `xml:"rollcall-vote"`
	VoteMetadata HouseVoteMetadata `xml:"vote-metadata"`
	VoteTotals   HouseVoteTotals   `xml:"vote-totals"`
	VoteData     HouseVoteData     `xml:"vote-data"`
}

type HouseVoteMetadata struct {
	Majority     string `xml:"majority"`
	Congress     int    `xml:"congress"`
	Session      string `xml:"session"`
	Chamber      string `xml:"chamber"`
	RollCallNum  int    `xml:"rollcall-num"`
	LegisNum     string `xml:"legis-num"`
	VoteQuestion string `xml:"vote-question"`
	VoteType     string `xml:"vote-type"`
	VoteResult   string `xml:"vote-result"`
	ActionDate   string `xml:"action-date"`
	ActionTime   string `xml:"action-time"`
	VoteDesc     string `xml:"vote-desc"`
}

type HouseVoteTotals struct {
	TotalsByParty []HouseTotalsTally `xml:"totals-by-party"`
}

type HouseTotalsTally struct {
	Party          string `xml:"party"`
	YeaTotal       int    `xml:"yea-total"`
	NayTotal       int    `xml:"nay-total"`
	PresentTotal   int    `xml:"present-total"`
	NotVotingTotal int    `xml:"not-voting-total"`
}

type HouseVoteData struct {
	RecordedVotes []HouseRecordedVote `xml:"recorded-vote"`
}

type HouseRecordedVote struct {
	Legislator HouseLegislator `xml:"legislator"`
	Vote       string          `xml:"vote"`
}

type HouseLegislator struct {
	NameID    string `xml:"name-id,attr"`
	SortField string `xml:"sort-field,attr"`
	UnaccName string `xml:"unaccented-name,attr"`
	Party     string `xml:"party,attr"`
	State     string `xml:"state,attr"`
	Role      string `xml:"role,attr"`
}

// parseHouseVote fetches and parses a House vote XML from clerk.house.gov
func (a *CongressAPI) parseHouseVote(ctx context.Context, url string) (*RollCallVote, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "https://legislation.support/")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var houseVote HouseRollCallVote
	dec := xml.NewDecoder(resp.Body)
	if err := dec.Decode(&houseVote); err != nil {
		return nil, err
	}

	return houseVote.ToRollCallVote(), nil
}

// ToRollCallVote converts a House vote to the unified RollCallVote format
func (h *HouseRollCallVote) ToRollCallVote() *RollCallVote {
	vote := &RollCallVote{
		Congress:            h.VoteMetadata.Congress,
		VoteNumber:          h.VoteMetadata.RollCallNum,
		VoteDate:            h.VoteMetadata.ActionDate + " " + h.VoteMetadata.ActionTime,
		VoteQuestionText:    h.VoteMetadata.VoteQuestion,
		VoteResult:          h.VoteMetadata.VoteResult,
		Question:            h.VoteMetadata.VoteQuestion,
		VoteTitle:           h.VoteMetadata.VoteDesc,
		MajorityRequirement: h.VoteMetadata.Majority,
	}

	// Convert vote totals - sum across all parties
	for _, tally := range h.VoteTotals.TotalsByParty {
		vote.Count.Yeas += tally.YeaTotal
		vote.Count.Nays += tally.NayTotal
		vote.Count.Present += tally.PresentTotal
		vote.Count.Absent += tally.NotVotingTotal
	}

	// Convert members
	for _, rv := range h.VoteData.RecordedVotes {
		member := VoteXMLMember{
			MemberFull:  rv.Legislator.UnaccName,
			LastName:    rv.Legislator.SortField,
			Party:       rv.Legislator.Party,
			State:       rv.Legislator.State,
			VoteCast:    rv.Vote,
			LISMemberID: rv.Legislator.NameID,
		}
		vote.Members = append(vote.Members, member)
	}

	return vote
}
