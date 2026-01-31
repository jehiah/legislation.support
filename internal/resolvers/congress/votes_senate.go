package congress

import (
	"context"
	"encoding/xml"
	"net/http"
)

// SenateRollCallVote represents the XML structure for Senate votes from senate.gov
// Example: https://www.senate.gov/legislative/LIS/roll_call_votes/vote1191/vote_119_1_00333.xml
type SenateRollCallVote struct {
	XMLName             xml.Name        `xml:"roll_call_vote"`
	Congress            int             `xml:"congress"`
	Session             int             `xml:"session"`
	CongressYear        int             `xml:"congress_year"`
	VoteNumber          int             `xml:"vote_number"`
	VoteDate            string          `xml:"vote_date"`
	ModifyDate          string          `xml:"modify_date"`
	VoteQuestionText    string          `xml:"vote_question_text"`
	VoteDocumentText    string          `xml:"vote_document_text"`
	VoteResultText      string          `xml:"vote_result_text"`
	Question            string          `xml:"question"`
	VoteTitle           string          `xml:"vote_title"`
	MajorityRequirement string          `xml:"majority_requirement"`
	VoteResult          string          `xml:"vote_result"`
	Document            VoteDocument    `xml:"document"`
	Amendment           VoteAmendment   `xml:"amendment"`
	Count               VoteCount       `xml:"count"`
	TieBreaker          VoteTieBreaker  `xml:"tie_breaker"`
	Members             []VoteXMLMember `xml:"members>member"`
}

type VoteDocument struct {
	DocumentCongress   int    `xml:"document_congress"`
	DocumentType       string `xml:"document_type"`
	DocumentNumber     string `xml:"document_number"`
	DocumentName       string `xml:"document_name"`
	DocumentTitle      string `xml:"document_title"`
	DocumentShortTitle string `xml:"document_short_title"`
}

type VoteAmendment struct {
	AmendmentNumber                       string `xml:"amendment_number"`
	AmendmentToAmendmentNumber            string `xml:"amendment_to_amendment_number"`
	AmendmentToAmendmentToAmendmentNumber string `xml:"amendment_to_amendment_to_amendment_number"`
	AmendmentToDocumentNumber             string `xml:"amendment_to_document_number"`
	AmendmentToDocumentShortTitle         string `xml:"amendment_to_document_short_title"`
	AmendmentPurpose                      string `xml:"amendment_purpose"`
}

type VoteCount struct {
	Yeas    int `xml:"yeas"`
	Nays    int `xml:"nays"`
	Present int `xml:"present"`
	Absent  int `xml:"absent"`
}

type VoteTieBreaker struct {
	ByWhom         string `xml:"by_whom"`
	TieBreakerVote string `xml:"tie_breaker_vote"`
}

type VoteXMLMember struct {
	MemberFull  string `xml:"member_full"`
	LastName    string `xml:"last_name"`
	FirstName   string `xml:"first_name"`
	Party       string `xml:"party"`
	State       string `xml:"state"`
	VoteCast    string `xml:"vote_cast"`
	LISMemberID string `xml:"lis_member_id"`
}

// parseSenateVote fetches and parses a Senate vote XML from senate.gov
func (a *CongressAPI) parseSenateVote(ctx context.Context, url string) (*RollCallVote, error) {
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

	var senateVote SenateRollCallVote
	dec := xml.NewDecoder(resp.Body)
	if err := dec.Decode(&senateVote); err != nil {
		return nil, err
	}

	return senateVote.ToRollCallVote(), nil
}

// ToRollCallVote converts a Senate vote to the unified RollCallVote format
func (s *SenateRollCallVote) ToRollCallVote() *RollCallVote {
	return &RollCallVote{
		Congress:            s.Congress,
		Session:             s.Session,
		CongressYear:        s.CongressYear,
		VoteNumber:          s.VoteNumber,
		VoteDate:            s.VoteDate,
		ModifyDate:          s.ModifyDate,
		VoteQuestionText:    s.VoteQuestionText,
		VoteDocumentText:    s.VoteDocumentText,
		VoteResultText:      s.VoteResultText,
		Question:            s.Question,
		VoteTitle:           s.VoteTitle,
		MajorityRequirement: s.MajorityRequirement,
		VoteResult:          s.VoteResult,
		Document:            s.Document,
		Amendment:           s.Amendment,
		Count:               s.Count,
		TieBreaker:          s.TieBreaker,
		Members:             s.Members,
	}
}
