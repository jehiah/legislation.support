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
	a := New(legislature.Body{}, os.Getenv("NY_SENATE_TOKEN"))
	b, err := a.GetBill(context.Background(), "2021", "S5130")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%#v", b)
}

func TestLookup(t *testing.T) {

	type testCase struct {
		url string
		// found bool
	}
	tests := []testCase{
		{"https://www.nysenate.gov/legislation/bills/2019/s2892"},
	}
	a := New(legislature.Body{}, os.Getenv("NY_SENATE_TOKEN"))
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
