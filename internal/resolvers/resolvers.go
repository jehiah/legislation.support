package resolvers

import (
	"context"
	"net/url"
	"os"

	"github.com/jehiah/legislation.support/internal/legislature"
	"github.com/jehiah/legislation.support/internal/resolvers/congress"
	"github.com/jehiah/legislation.support/internal/resolvers/nyc"
	"github.com/jehiah/legislation.support/internal/resolvers/nysenate"
)

var (
	NYCCouncil = legislature.Body{
		ID:         "nyc",
		Name:       "NYC City Council",
		DisplayID:  "NYC-Council",
		Location:   "New York City",
		URL:        "https://council.nyc.gov/",
		MemberName: "Council Member",
		Sort:       legislature.GenericLegislationSort,
	}
	NYSenate = legislature.Body{
		ID:         "nysenate",
		Bicameral:  "ny-assembly",
		UpperHouse: true,
		Name:       "NY Senate",
		DisplayID:  "NY-Senate",
		Location:   "New York",
		URL:        "https://www.nysenate.gov/",
		MemberName: "Senator",
		Sort:       nysenate.LegislationSort,
	}
	NYAssembly = legislature.Body{
		ID:         "ny-assembly",
		Bicameral:  "nysenate",
		Name:       "NY Assembly",
		DisplayID:  "NY-Assembly",
		Location:   "New York",
		URL:        "https://assembly.state.ny.us/",
		MemberName: "Assembly Member",
		Sort:       nysenate.LegislationSort,
	}
	USHouse = legislature.Body{
		ID:         "us-house",
		Bicameral:  "us-senate",
		Name:       "US House of Representatives",
		DisplayID:  "US-House",
		Location:   "United States",
		URL:        "https://www.house.gov/",
		MemberName: "Representative",
		Sort:       legislature.GenericLegislationSort,
	}
	USSenate = legislature.Body{
		ID:         "us-senate",
		Bicameral:  "us-house",
		UpperHouse: true,
		Name:       "US Senate",
		DisplayID:  "US-Senate",
		Location:   "United States",
		URL:        "https://www.senate.gov/",
		MemberName: "Senator",
		Sort:       legislature.GenericLegislationSort,
	}
)

var Resolvers = legislature.Resolvers{
	nyc.New(NYCCouncil),
	nysenate.NewNYSenate(NYSenate, os.Getenv("NY_SENATE_TOKEN")),
	nysenate.NewNYAssembly(NYAssembly, os.Getenv("NY_SENATE_TOKEN")),
	congress.NewHouse(USHouse, os.Getenv("CONGRESS_GOV_APIKEY")),
	congress.NewSenate(USSenate, os.Getenv("CONGRESS_GOV_APIKEY")),
}

func Lookup(ctx context.Context, u *url.URL) (*legislature.Legislation, error) {
	return Resolvers.Lookup(ctx, u)
}

func SupportedDomains() []string {
	return Resolvers.SupportedDomains()
}

var Bodies map[legislature.BodyID]legislature.Body = map[legislature.BodyID]legislature.Body{
	NYCCouncil.ID: NYCCouncil,
	NYSenate.ID:   NYSenate,
	NYAssembly.ID: NYAssembly,
	USHouse.ID:    USHouse,
	USSenate.ID:   USSenate,
}

func IsValidBodyID(ID legislature.BodyID) bool {
	_, ok := Bodies[ID]
	return ok
}

func IsBicameral(b legislature.BodyID) bool {
	return Bodies[b].Bicameral != ""
}
