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

}
