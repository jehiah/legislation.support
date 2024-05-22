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

func TestGetMembers(t *testing.T) {
	a := NewAPI(os.Getenv("NY_SENATE_TOKEN"))
	m, err := a.GetMembers(context.Background(), Sessions[0], "senate")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%#v", m)
	found := false
	for _, s := range m {
		if s.ShortName == "HOYLMAN-SIGAL" {
			found = true
		}
	}
	if !found {
		t.Error("expected HOYLMAN-SIGAL")
	}
}

func TestNYSenateLookup(t *testing.T) {

	type testCase struct {
		url string
		// found bool
	}
	tests := []testCase{
		{"https://www.nysenate.gov/legislation/bills/2019/s2892"},
		{"https://www.nysenate.gov/legislation/bills/2023/s2714/"},
		{"https://www.nysenate.gov/legislation/bills/2023/S1724/amendment/A"},
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
		legislature.BodyID
	}

	tests := []testCase{
		{"https://www.nysenate.gov/legislation/bills/2021/A4854", legislature.BodyID("A")},
		{"https://assembly.state.ny.us/leg/?default_fld=&bn=A04854&term=2021&Summary=Y&Actions=Y&Text=Y&Committee%26nbspVotes=Y&Floor%26nbspVotes=Y", legislature.BodyID("A")},
		{"https://nyassembly.gov/leg/?default_fld=&leg_video=&bn=A08273&term=2023&Summary=Y&Memo=Y&Chamber%26nbspVideo%2FTranscript=Y", legislature.BodyID("A")},
		{"https://assembly.state.ny.us/leg/?default_fld=&bn=S01982&term=2023", legislature.BodyID("S")},
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
