package nyc

import (
	"context"
	"net/url"
	"testing"
)

func TestLookupLegistarLegislationDetail(t *testing.T) {
	var n NYC
	u, _ := url.Parse("https://legistar.council.nyc.gov/LegislationDetail.aspx?ID=3704308&GUID=C7C66706-1DAD-4F98-93AC-97593540092E")
	l, err := n.LookupLegistarLegislationDetail(context.Background(), u)
	if err != nil {
		t.Fatal(err)
	}
	if l == nil {
		t.Fatal("missing url")
	}
	expected := "https://intro.nyc/1141-2018"
	if l.String() != expected {
		t.Errorf("got %s expected %s", l.String(), expected)
	}
}
