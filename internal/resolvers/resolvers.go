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
		Sort:      legislature.GenericLegislationSort,
	}
	NYSenate = legislature.Body{
		ID:        "nysenate",
		Bicameral: "ny-assembly",
		Name:      "NY Senate",
		DisplayID: "NY-Senate",
		Location:  "New York",
		URL:       "https://www.nysenate.gov/",
		Sort:      nysenate.LegislationSort,
	}
	NYAssembly = legislature.Body{
		ID:        "ny-assembly",
		Bicameral: "nysenate",
		Name:      "NY Assembly",
		DisplayID: "NY-Assembly",
		Location:  "New York",
		URL:       "https://assembly.state.ny.us/",
		Sort:      nysenate.LegislationSort,
	}
)

var Resolvers = legislature.Resolvers{
	nyc.New(NYCCouncil),
	nysenate.NewNYSenate(NYSenate, os.Getenv("NY_SENATE_TOKEN")),
	nysenate.NewNYAssembly(NYAssembly, os.Getenv("NY_SENATE_TOKEN")),
}

var Bodies map[legislature.BodyID]legislature.Body = map[legislature.BodyID]legislature.Body{
	NYCCouncil.ID: NYCCouncil,
	NYSenate.ID:   NYSenate,
	NYAssembly.ID: NYAssembly,
}

func IsValidBodyID(ID legislature.BodyID) bool {
	_, ok := Bodies[ID]
	return ok
}
