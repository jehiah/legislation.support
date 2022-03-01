package nyc

import (
	"net/url"

	"github.com/jehiah/legislation.support/internal/legislature"
)

type NYC struct {
}

func New() *NYC {
	return &NYC{}
}

func (n NYC) Lookup(u *url.URL) (*legislature.Legislation, error) {
	switch u.Hostname() {
	case "legistar.council.nyc.gov":
		if u.Path != "/LegislationDetail.aspx" {
			return nil, legislature.ErrNotFound
		}
		// todo
	case "intro.nyc":
	default:
		return nil, legislature.ErrNotFound
	}
	return nil, legislature.ErrNotFound
}
