package legislature

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"reflect"
	"testing"
	"time"
)

type testResolver struct {
	body             Body
	supportedDomains []string
	lookupResult     *Legislation
	lookupErr        error
	lookupCalls      int
}

func (r *testResolver) Lookup(ctx context.Context, u *url.URL) (*Legislation, error) {
	r.lookupCalls++
	return r.lookupResult, r.lookupErr
}

func (r *testResolver) Refresh(context.Context, LegislationID) (*Legislation, error) {
	return nil, nil
}

func (r *testResolver) Body() Body { return r.body }

func (r *testResolver) Scorecard(context.Context, []Scorable) (*Scorecard, error) {
	return nil, nil
}

func (r *testResolver) Members(context.Context, Session) ([]Member, error) { return nil, nil }

func (r *testResolver) Link(LegislationID) *url.URL { return nil }

func (r *testResolver) DisplayID(LegislationID) string { return "" }

func (r *testResolver) SupportedDomains() []string { return r.supportedDomains }

func TestCalculateSponsorChanges(t *testing.T) {
	tests := []struct {
		name string
		a, b Legislation
		want []SponsorChange
	}{
		{
			name: "no changes",
		},
		{
			name: "add a sponsor",
			b: Legislation{
				Sponsors: []Member{
					{NumericID: 1},
				},
			},
			want: []SponsorChange{
				{Member: Member{NumericID: 1}},
			},
		},
		{
			name: "change details",
			a: Legislation{
				Sponsors: []Member{
					{NumericID: 1},
				},
			},
			b: Legislation{
				Sponsors: []Member{
					{NumericID: 1, FullName: "New Name"},
				},
			},
		},
		// TODO: Add test cases.
	}
	for i, tc := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			t.Log(tc.name)
			got := CalculateSponsorChanges(tc.a, tc.b)
			for i := range got {
				got[i].Date = time.Time{}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("CalculateSponsorChanges() = %#v, want %v", got, tc.want)
			}
		})
	}
}

func TestResolversLookupSkipsUnsupportedDomains(t *testing.T) {
	match := &testResolver{
		supportedDomains: []string{"example.com"},
		lookupResult:     &Legislation{ID: "matched"},
	}
	skipped := &testResolver{
		supportedDomains: []string{"other.example"},
		lookupErr:        errors.New("should not be called"),
	}

	legislation, err := Resolvers{skipped, match}.Lookup(t.Context(), &url.URL{Scheme: "https", Host: "example.com", Path: "/bill"})
	if err != nil {
		t.Fatalf("Lookup() error = %v", err)
	}
	if legislation == nil || legislation.ID != "matched" {
		t.Fatalf("Lookup() = %#v, want matched legislation", legislation)
	}
	if skipped.lookupCalls != 0 {
		t.Fatalf("skipped resolver lookupCalls = %d, want 0", skipped.lookupCalls)
	}
	if match.lookupCalls != 1 {
		t.Fatalf("match resolver lookupCalls = %d, want 1", match.lookupCalls)
	}
}

func TestResolversLookupReturnsErrorFromMatchingResolver(t *testing.T) {
	wantErr := errors.New("lookup failed")
	matching := &testResolver{
		supportedDomains: []string{"example.com"},
		lookupErr:        wantErr,
	}
	nonMatching := &testResolver{
		supportedDomains: []string{"other.example"},
		lookupResult:     &Legislation{ID: "wrong"},
	}

	legislation, err := Resolvers{nonMatching, matching}.Lookup(t.Context(), &url.URL{Scheme: "https", Host: "example.com", Path: "/bill"})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Lookup() error = %v, want %v", err, wantErr)
	}
	if legislation != nil {
		t.Fatalf("Lookup() = %#v, want nil", legislation)
	}
	if nonMatching.lookupCalls != 0 {
		t.Fatalf("nonMatching resolver lookupCalls = %d, want 0", nonMatching.lookupCalls)
	}
	if matching.lookupCalls != 1 {
		t.Fatalf("matching resolver lookupCalls = %d, want 1", matching.lookupCalls)
	}
}
