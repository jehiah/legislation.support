package nysenate

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"

	"github.com/jehiah/legislation.support/internal/legislature"
)

func TestGetBill(t *testing.T) {
	a := NewAPI(os.Getenv("NY_SENATE_TOKEN"))
	b, err := a.GetBill(context.Background(), "2021", "S5130")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%#v", b)
}

func TestNYSenateLookup(t *testing.T) {

	type testCase struct {
		url string
		// found bool
	}
	tests := []testCase{
		{"https://www.nysenate.gov/legislation/bills/2019/s2892"},
		{"https://www.nysenate.gov/legislation/bills/2021/s4547/amendment/a"},
	}
	a := NewNYSenate(legislature.Body{}, os.Getenv("NY_SENATE_TOKEN"))
	for i, tc := range tests {
		tc := tc
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			t.Logf("%#v", tc)
			u, err := url.Parse(tc.url)
			if err != nil {
				t.Fatal(err)
			}
			b, err := a.Lookup(context.Background(), u)
			if err != nil {
				t.Fatal(err)
			}
			if b == nil {
				t.Fatal("exptected URL")
			}
		})
	}
}
func TestNYAssemblyLookup(t *testing.T) {

	type testCase struct {
		url string
		// found bool
	}
	tests := []testCase{
		{"https://www.nysenate.gov/legislation/bills/2021/A4854"},
		{"https://assembly.state.ny.us/leg/?default_fld=&bn=A04854&term=2021&Summary=Y&Actions=Y&Text=Y&Committee%26nbspVotes=Y&Floor%26nbspVotes=Y"},
	}
	a := NewNYAssembly(legislature.Body{}, os.Getenv("NY_SENATE_TOKEN"))
	for i, tc := range tests {
		tc := tc
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			t.Logf("%#v", tc)
			u, err := url.Parse(tc.url)
			if err != nil {
				t.Fatal(err)
			}
			b, err := a.Lookup(context.Background(), u)
			if err != nil {
				t.Fatal(err)
			}
			if b == nil {
				t.Fatal("exptected URL")
			}
		})
	}
}
