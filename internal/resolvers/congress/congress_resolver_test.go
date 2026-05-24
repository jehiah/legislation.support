package congress

import (
	"net/url"
	"testing"

	"github.com/jehiah/legislation.support/internal/legislature"
)

func TestHouseLookupIgnoresNonBillCongressURL(t *testing.T) {
	h := House{
		body: legislature.Body{ID: "us-house"},
		api:  &CongressAPI{},
	}
	u, err := url.Parse("https://www.congress.gov/search")
	if err != nil {
		t.Fatal(err)
	}

	legislation, err := h.Lookup(t.Context(), u)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if legislation != nil {
		t.Fatalf("Lookup() = %#v, want nil", legislation)
	}
}

func TestSenateLookupIgnoresNonBillCongressURL(t *testing.T) {
	s := Senate{
		body: legislature.Body{ID: "us-senate"},
		api:  &CongressAPI{},
	}
	u, err := url.Parse("https://www.congress.gov/search")
	if err != nil {
		t.Fatal(err)
	}

	legislation, err := s.Lookup(t.Context(), u)
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if legislation != nil {
		t.Fatalf("Lookup() = %#v, want nil", legislation)
	}
}
