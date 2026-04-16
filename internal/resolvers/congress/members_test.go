package congress_test

import (
	"context"
	"testing"

	"github.com/jehiah/legislation.support/internal/legislature"
	"github.com/jehiah/legislation.support/internal/resolvers/congress"
)

func TestCongress_Members(t *testing.T) {
	c := congress.NewAPI("")
	members, err := c.Members(context.Background(), legislature.Session{StartYear: 2025})
	if err != nil {
		t.Fatal(err)
	}
	if len(members) == 0 {
		t.Fatalf("expected members, got none")
	}

	m := findMember(members, "G000594")
	if m == nil {
		t.Fatalf("expected to find member G000594, but did not")
	}
	if got := m.ToLegislatureMember().FullName; got != "Tony Gonzales" {
		t.Fatalf("member %s full name = %q, want %q", m.BioguideID, got, "Tony Gonzales")
	}
}

func findMember(members []congress.Member, bioguideID string) *congress.Member {
	for _, m := range members {
		if m.BioguideID == bioguideID {
			return &m
		}
	}
	return nil
}
