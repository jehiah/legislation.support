package nysenate

import (
	"context"
	"testing"
)

func TestAssemblyVotes(t *testing.T) {
	ctx := context.Background()
	m, err := api.GetMembers(ctx, Sessions.Find(2021), assemblyChamber)
	if err != nil {
		t.Fatal(err)
	}

	bill, err := api.AssemblyVotes(ctx, m, "2021", "A09275")
	if err != nil {
		t.Fatal(err)
	}
	// t.Logf("%#v", bill.Votes.Items)
	if len(bill.Votes.Items) != 4 {
		t.Fatalf("expected 4 votes got %d", len(bill.Votes.Items))
	}
	votes := bill.GetVotes()
	for i, v := range votes {
		if v.MemberID == 0 {
			t.Logf("[%d] unknown member %#v", i, v)
		}
	}

	// this bill has "Held for consideration" votes that should be skipped
	bill, err = api.AssemblyVotes(ctx, m, "2023", "A06141")
	if err != nil {
		t.Fatal(err)
	}
	// t.Logf("%#v", bill.Votes.Items)
	if len(bill.Votes.Items) < 1 {
		t.Fatalf("expected 1 votes got %d", len(bill.Votes.Items))
	}
	if bill.Votes.Items[0].VoteType != "Held for Consideration" {
		t.Fatalf("expected Held for Consideration got %s", bill.Votes.Items[0].VoteType)
	}

	// this bill has "Held for consideration" votes that should be skipped
	m, err = api.GetMembers(ctx, Sessions.Find(2025), assemblyChamber)
	if err != nil {
		t.Fatalf("%s", err)
	}
	bill, err = api.AssemblyVotes(ctx, m, "2025", "A03665")
	if err != nil {
		t.Fatal(err)
	}
	if len(bill.Votes.Items) != 3 {
		t.Fatalf("expected 3 votes got %d", len(bill.Votes.Items))
	}
	votes = bill.GetVotes()
	for i, v := range votes {
		if v.MemberID == 0 {
			t.Logf("[%d] unknown member %#v", i, v)
		}
	}
	for _, v := range bill.Votes.Items[2:] {
		t.Logf("%#v", v)
		if len(v.GetVotes()) != 149 {
			t.Errorf("got %d votes expected 149", len(v.GetVotes()))
		}
		for _, m := range v.GetVotes() {
			t.Logf("member %s %d %s", m.ShortName, m.MemberID, m.Vote)
			if m.MemberID == 0 {
				t.Logf("unknown member %s", m.ShortName)
			}
		}
	}
	// t.Logf("%#v", bill.Votes.Items)

}
