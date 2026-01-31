package congress

import (
	"context"
	"fmt"
	"strings"
)

// RollCallVote represents a unified structure for both Senate and House votes
type RollCallVote struct {
	Congress            int
	Session             int
	CongressYear        int
	VoteNumber          int
	VoteDate            string
	ModifyDate          string
	VoteQuestionText    string
	VoteDocumentText    string
	VoteResultText      string
	Question            string
	VoteTitle           string
	MajorityRequirement string
	VoteResult          string
	Document            VoteDocument
	Amendment           VoteAmendment
	Count               VoteCount
	TieBreaker          VoteTieBreaker
	Members             []VoteXMLMember
}

// GetVoteXML fetches and parses the XML vote record from the given URL
// Detects whether it's a House or Senate vote based on the domain and parses accordingly
func (a *CongressAPI) GetVoteXML(ctx context.Context, url string) (*RollCallVote, error) {
	// Determine which parser to use based on URL
	if strings.Contains(url, "clerk.house.gov") {
		return a.parseHouseVote(ctx, url)
	} else if strings.Contains(url, "senate.gov") {
		return a.parseSenateVote(ctx, url)
	}

	return nil, fmt.Errorf("unsupported vote URL domain: %s", url)
}
