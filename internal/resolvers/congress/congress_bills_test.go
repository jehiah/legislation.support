package congress

import (
	"fmt"
	"testing"

	"github.com/jehiah/legislation.support/internal/legislature"
)

func TestParseBillID(t *testing.T) {
	tests := []struct {
		id           legislature.LegislationID
		wantCongress int
		wantBillType string
		wantNumber   string
		wantErr      bool
	}{
		{"118-HR1234", 118, "HR", "1234", false},
		{"117-S874", 117, "S", "874", false},
		{"118-HJRES45", 118, "HJRES", "45", false},
		{"118-SJRES12", 118, "SJRES", "12", false},
		{"118-HCONRES3", 118, "HCONRES", "3", false},
		{"118-SCONRES7", 118, "SCONRES", "7", false},
		{"118-HRES100", 118, "HRES", "100", false},
		{"118-SRES200", 118, "SRES", "200", false},
		{"invalid", 0, "", "", true},
		{"118", 0, "", "", true},
		{"118-", 0, "", "", true},
		{"118-xyz123", 0, "", "", true},
	}

	for _, tt := range tests {
		t.Run(string(tt.id), func(t *testing.T) {
			congress, billType, number, err := parseBillID(tt.id)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseBillID(%q) error = %v, wantErr %v", tt.id, err, tt.wantErr)
				return
			}
			if err != nil {
				return
			}
			if congress != tt.wantCongress {
				t.Errorf("parseBillID(%q) congress = %v, want %v", tt.id, congress, tt.wantCongress)
			}
			if billType != tt.wantBillType {
				t.Errorf("parseBillID(%q) billType = %v, want %v", tt.id, billType, tt.wantBillType)
			}
			if number != tt.wantNumber {
				t.Errorf("parseBillID(%q) number = %v, want %v", tt.id, number, tt.wantNumber)
			}
		})
	}
}

func TestFormatDisplayID(t *testing.T) {
	tests := []struct {
		billType string
		number   string
		want     string
	}{
		{"HR", "1234", "H.R. 1234"},
		{"S", "874", "S. 874"},
		{"HJRES", "45", "H.J.Res. 45"},
		{"SJRES", "12", "S.J.Res. 12"},
		{"HCONRES", "3", "H.Con.Res. 3"},
		{"SCONRES", "7", "S.Con.Res. 7"},
		{"HRES", "100", "H.Res. 100"},
		{"SRES", "200", "S.Res. 200"},
	}

	for _, tt := range tests {
		t.Run(tt.billType+tt.number, func(t *testing.T) {
			got := formatDisplayID(tt.billType, tt.number)
			if got != tt.want {
				t.Errorf("formatDisplayID(%q, %q) = %q, want %q", tt.billType, tt.number, got, tt.want)
			}
		})
	}
}

func TestBillTypeToName(t *testing.T) {
	tests := []struct {
		billType string
		want     string
	}{
		{"HR", "house-bill"},
		{"S", "senate-bill"},
		{"HJRES", "house-joint-resolution"},
		{"SJRES", "senate-joint-resolution"},
		{"HCONRES", "house-concurrent-resolution"},
		{"SCONRES", "senate-concurrent-resolution"},
		{"HRES", "house-resolution"},
		{"SRES", "senate-resolution"},
	}

	for _, tt := range tests {
		t.Run(tt.billType, func(t *testing.T) {
			got := billTypeToName(tt.billType)
			if got != tt.want {
				t.Errorf("billTypeToName(%q) = %q, want %q", tt.billType, got, tt.want)
			}
		})
	}
}

func TestCongressNumber(t *testing.T) {
	tests := []struct {
		session legislature.Session
		want    int
	}{
		{legislature.Session{StartYear: 2025}, 119},
		{legislature.Session{StartYear: 2023}, 118},
		{legislature.Session{StartYear: 2021}, 117},
		{legislature.Session{StartYear: 2019}, 116},
		{legislature.Session{StartYear: 1789}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.session.String(), func(t *testing.T) {
			got := congressNumber(tt.session)
			if got != tt.want {
				t.Errorf("congressNumber(%v) = %d, want %d", tt.session, got, tt.want)
			}
		})
	}
}

func TestGetBill(t *testing.T) {
	tests := []struct {
		name          string
		congress      int
		billType      string
		number        string
		wantNumber    string
		wantType      string
		wantCongress  string
		wantOrigin    string
		wantDisplayID string
		wantLegType   legislature.LegislationType
	}{
		{
			name:          "H.Res. 996 (119th)",
			congress:      119,
			billType:      "HRES",
			number:        "996",
			wantNumber:    "996",
			wantType:      "HRES",
			wantCongress:  "119",
			wantOrigin:    "House",
			wantDisplayID: "H.Res. 996",
			wantLegType:   legislature.ResolutionType,
		},
		{
			name:          "H.R. 7233 (119th)",
			congress:      119,
			billType:      "HR",
			number:        "7233",
			wantNumber:    "7233",
			wantType:      "HR",
			wantCongress:  "119",
			wantOrigin:    "House",
			wantDisplayID: "H.R. 7233",
			wantLegType:   legislature.BillType,
		},
	}

	api := NewAPI("")
	session := Sessions.Find(2025)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bill, err := api.GetBill(t.Context(), tt.congress, tt.billType, tt.number)
			if err != nil {
				t.Fatal(err)
			}
			if bill == nil {
				t.Fatal("expected bill, got nil")
			}

			// Verify bill fields
			if bill.Number != tt.wantNumber {
				t.Errorf("bill.Number = %q, want %q", bill.Number, tt.wantNumber)
			}
			if bill.Type != tt.wantType {
				t.Errorf("bill.Type = %q, want %q", bill.Type, tt.wantType)
			}
			if bill.Congress.String() != tt.wantCongress {
				t.Errorf("bill.Congress = %q, want %q", bill.Congress, tt.wantCongress)
			}
			if bill.OriginChamber != tt.wantOrigin {
				t.Errorf("bill.OriginChamber = %q, want %q", bill.OriginChamber, tt.wantOrigin)
			}
			if bill.Title == "" {
				t.Error("bill.Title is empty")
			}

			// Verify ID generation
			expectedID := legislature.LegislationID(fmt.Sprintf("%s-%s%s", tt.wantCongress, tt.wantType, tt.wantNumber))
			if bill.ID() != expectedID {
				t.Errorf("bill.ID() = %q, want %q", bill.ID(), expectedID)
			}

			// Verify conversion to Legislation
			leg, err := bill.ToLegislation("us-house", session)
			if err != nil {
				t.Fatalf("ToLegislation() error = %v", err)
			}
			if leg.DisplayID != tt.wantDisplayID {
				t.Errorf("leg.DisplayID = %q, want %q", leg.DisplayID, tt.wantDisplayID)
			}
			if leg.Type != tt.wantLegType {
				t.Errorf("leg.Type = %q, want %q", leg.Type, tt.wantLegType)
			}
			if leg.Session != session {
				t.Errorf("leg.Session = %v, want %v", leg.Session, session)
			}
			if leg.URL == "" {
				t.Error("leg.URL is empty")
			}

			t.Logf("Successfully fetched %s", tt.name)
			t.Logf("  Title: %s", leg.Title)
			t.Logf("  Status: %s", leg.Status)
			t.Logf("  Sponsors: %d", len(leg.Sponsors))
			t.Logf("  URL: %s", leg.URL)
		})
	}
}

func TestGetCosponsors(t *testing.T) {
	api := NewAPI("")

	// Test with H.R. 7233 from 119th Congress
	cosponsors, err := api.GetCosponsors(t.Context(), 119, "hr", "7233")
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("Found %d cosponsors for H.R. 7233", len(cosponsors))

	// Verify basic structure if we have cosponsors
	if len(cosponsors) > 0 {
		first := cosponsors[0]
		if first.BioguideID == "" {
			t.Error("first cosponsor BioguideID is empty")
		}
		if first.FullName == "" {
			t.Error("first cosponsor FullName is empty")
		}
		t.Logf("First cosponsor: %s (%s)", first.FullName, first.BioguideID)
	}
}

func TestGetActions(t *testing.T) {
	api := NewAPI("")

	// Test with H.R. 1 from 119th Congress (should have many actions)
	actions, err := api.GetActions(t.Context(), 119, "hr", "1")
	if err != nil {
		t.Fatal(err)
	}

	if len(actions) == 0 {
		t.Fatal("expected actions, got none")
	}

	t.Logf("Found %d actions for H.R. 1", len(actions))

	// Verify basic structure
	first := actions[0]
	if first.ActionDate == "" {
		t.Error("first action ActionDate is empty")
	}
	if first.Text == "" {
		t.Error("first action Text is empty")
	}

	t.Logf("First action: %s - %s", first.ActionDate, first.Text)

	// Check for recorded votes
	var votesFound int
	for _, action := range actions {
		if len(action.RecordedVotes) > 0 {
			votesFound += len(action.RecordedVotes)
			for _, rv := range action.RecordedVotes {
				t.Logf("Recorded vote: %s roll %d on %s", rv.Chamber, rv.RollNumber, rv.Date)
			}
		}
	}

	if votesFound > 0 {
		t.Logf("Found %d recorded votes in actions", votesFound)
	}
}

func TestGetVoteXML(t *testing.T) {
	tests := []struct {
		name       string
		url        string
		wantCongress int
		wantVoteNum int
	}{
		{
			name:       "House Vote",
			url:        "https://clerk.house.gov/evs/2025/roll190.xml",
			wantCongress: 119,
			wantVoteNum: 190,
		},
		{
			name:       "Senate Vote",
			url:        "https://www.senate.gov/legislative/LIS/roll_call_votes/vote1191/vote_119_1_00333.xml",
			wantCongress: 119,
			wantVoteNum: 333,
		},
	}

	api := NewAPI("")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			vote, err := api.GetVoteXML(t.Context(), tt.url)
			if err != nil {
				t.Fatal(err)
			}

			if vote == nil {
				t.Fatal("expected vote, got nil")
			}

			// Verify basic vote fields
			if vote.VoteNumber != tt.wantVoteNum {
				t.Errorf("vote.VoteNumber = %d, want %d", vote.VoteNumber, tt.wantVoteNum)
			}
			if vote.Congress != tt.wantCongress {
				t.Errorf("vote.Congress = %d, want %d", vote.Congress, tt.wantCongress)
			}
			if vote.VoteResult == "" {
				t.Error("vote.VoteResult is empty")
			}
			if vote.Question == "" {
				t.Error("vote.Question is empty")
			}

			t.Logf("Vote details:")
			t.Logf("  Congress: %d", vote.Congress)
			t.Logf("  Vote Number: %d", vote.VoteNumber)
			t.Logf("  Question: %s", vote.Question)
			t.Logf("  Result: %s", vote.VoteResult)
			t.Logf("  Yea: %d, Nay: %d, Present: %d, Absent: %d",
				vote.Count.Yeas, vote.Count.Nays,
				vote.Count.Present, vote.Count.Absent)

			if len(vote.Members) > 0 {
				t.Logf("  Total members voted: %d", len(vote.Members))
				t.Logf("  First vote: %s - %s", vote.Members[0].MemberFull, vote.Members[0].VoteCast)
			}
		})
	}
}
