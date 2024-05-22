package nysenate

import (
	"context"
	"os"
	"testing"
)

func TestAssemblyVotes(t *testing.T) {
	a := NewAPI(os.Getenv("NY_SENATE_TOKEN"))
	ctx := context.Background()
	m, err := a.GetMembers(ctx, Sessions[1], "assembly")
	if err != nil {
		t.Fatal(err)
	}

	bill, err := a.AssemblyVotes(ctx, m, "2021", "A09275")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%#v", bill.Votes.Items)
	if len(bill.Votes.Items) != 4 {
		t.Fatalf("expected 4 votes got %d", len(bill.Votes.Items))
	}
	votes := bill.GetVotes()
	for i, v := range votes {
		if v.MemberID == 0 {
			t.Logf("[%d] unknown member %#v", i, v)
		}
	}

}
