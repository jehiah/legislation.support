package nysenate

import (
	"context"
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
