package resolvers

import (
	"os"

	"github.com/jehiah/legislation.support/internal/legislature"
	"github.com/jehiah/legislation.support/internal/resolvers/nyc"
	"github.com/jehiah/legislation.support/internal/resolvers/nysenate"
)

var (
	NYCCouncil = legislature.Body{
		ID:       "nyc",
		Name:     "NYC City Council",
		Location: "New York City",
		URL:      "https://council.nyc.gov/",
	}
	NYSenate = legislature.Body{
		ID:       "nysenate",
		Name:     "NY Senate",
		Location: "New York",
		URL:      "https://www.nysenate.gov/",
	}
)

var Resolvers = legislature.Resolvers{
	nyc.New(NYCCouncil),
	nysenate.New(NYSenate, os.Getenv("NY_SENATE_TOKEN")),
}
