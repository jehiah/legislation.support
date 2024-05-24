package billtrack50

import (
	"context"
	"testing"
)

func TestBillOfficialDocument(t *testing.T) {
	api := BillTrack50{}
	u, err := api.billOfficialDocument(context.Background(), "1617018")
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("url: %s", u.String())
	if u.String() != "https://www.nysenate.gov/legislation/bills/2023/A6141" {
		t.Fatalf("unexpected url %s", u.String())
	}
}
