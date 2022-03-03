package resolvers

import (
	"github.com/jehiah/legislation.support/internal/legislature"
	"github.com/jehiah/legislation.support/internal/resolvers/nyc"
)

var (
	NYCCouncil = legislature.Body{
		ID:       "nyc",
		Name:     "NYC City Council",
		Location: "New York City",
		URL:      "https://council.nyc.gov/",
	}
)

var Resolvers = legislature.Resolvers{
	nyc.New(NYCCouncil),
}
