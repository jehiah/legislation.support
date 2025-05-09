package nyc

import (
	"context"
	"fmt"
	"net/url"
	"testing"
)

func TestLookupLegistarLegislationDetail(t *testing.T) {

	type testCase struct {
		URL      string
		Expected string
	}

	tests := []testCase{
		{
			URL:      "https://legistar.council.nyc.gov/LegislationDetail.aspx?ID=3704308&GUID=C7C66706-1DAD-4F98-93AC-97593540092E",
			Expected: "https://intro.nyc/1141-2018",
		},
		{
			URL:      "https://legistar.council.nyc.gov/LegislationDetail.aspx?ID=7086107&GUID=5DA2683D-D9B9-4633-B415-F75821C645D0",
			Expected: "https://intro.nyc/res-0707-2025",
		},
	}

	for i, tc := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			var n NYC
			u, err := url.Parse(tc.URL)
			if err != nil {
				t.Fatal(err)
			}
			l, err := n.LookupLegistarLegislationDetail(context.Background(), u)
			if err != nil {
				t.Fatal(err)
			}
			if l == nil {
				t.Fatal("missing url")
			}
			if l.String() != tc.Expected {
				t.Errorf("got %s expected %s", l.String(), tc.Expected)
			}
		})
	}
}
