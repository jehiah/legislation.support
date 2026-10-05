package nyc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
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

func TestIntroPattern(t *testing.T) {
	tests := []struct {
		path string
		want bool
	}{
		{path: "/1141-2018", want: true},
		{path: "/1141-2018+", want: true},
		{path: "/res-0707-2025+", want: true},
		{path: "/1141-2018++", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			if got := introPattern.MatchString(tc.path); got != tc.want {
				t.Errorf("introPattern.MatchString(%q) = %t, want %t", tc.path, got, tc.want)
			}
		})
	}
}

func TestIntroJSONTrimsPlusSuffix(t *testing.T) {
	var requestedPath string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPath = r.URL.Path
		_ = json.NewEncoder(w).Encode(map[string]string{"File": "Int 1141-2018"})
	}))
	defer server.Close()

	var n NYC
	d, err := n.IntroJSON(context.Background(), server.URL+"/1141-2018+")
	if err != nil {
		t.Fatal(err)
	}
	if d == nil || d.File != "Int 1141-2018" {
		t.Fatalf("got legislation %#v, want File %q", d, "Int 1141-2018")
	}
	if requestedPath != "/1141-2018.json" {
		t.Errorf("requested path = %q, want %q", requestedPath, "/1141-2018.json")
	}
}
