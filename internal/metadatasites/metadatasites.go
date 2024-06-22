package metadatasites

import (
	"context"
	"net/url"

	"github.com/jehiah/legislation.support/internal/legislature"
	"github.com/jehiah/legislation.support/internal/metadatasites/billtrack50"
)

var Sites = legislature.MetadataSites{
	billtrack50.BillTrack50{},
}

func Lookup(ctx context.Context, u *url.URL) (*url.URL, error) {
	return Sites.Lookup(ctx, u)
}
func SupportedDomains() []string {
	return Sites.SupportedDomains()
}
