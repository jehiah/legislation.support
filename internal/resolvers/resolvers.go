package resolvers

import (
	"os"

	"github.com/jehiah/legislation.support/internal/legislature"
	"github.com/jehiah/legislation.support/internal/resolvers/nyc"
	"github.com/jehiah/legislation.support/internal/resolvers/nysenate"
)

var (
	NYCCouncil = legislature.Body{
		ID:        "nyc",
		Name:      "NYC City Council",
		DisplayID: "NYC-Council",
		Location:  "New York City",
		URL:       "https://council.nyc.gov/",
	}
	NYSenate = legislature.Body{
		ID:        "nysenate",
		Name:      "NY Senate",
		DisplayID: "NY-Senate",
		Location:  "New York",
		URL:       "https://www.nysenate.gov/",
	}
	// NYAssembly = legislature.Body{
	// 	ID:        "ny-assembly",
	// 	Name:      "NY Assembly",
	// 	DisplayID: "NY-Assembly",
	// 	Location:  "New York",
	// 	URL:       "https://assembly.state.ny.us/",
	// }
)

var Resolvers = legislature.Resolvers{
	nyc.New(NYCCouncil),
	nysenate.New(NYSenate, os.Getenv("NY_SENATE_TOKEN")),
}

var Bodies map[legislature.BodyID]legislature.Body = map[legislature.BodyID]legislature.Body{
	NYCCouncil.ID: NYCCouncil,
	NYSenate.ID:   NYSenate,
}
