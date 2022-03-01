package main

import (
	"github.com/jehiah/legislation.support/internal/legislature"
	"github.com/jehiah/legislation.support/internal/resolvers/nyc"
)

var Bodies = []legislature.Body{
	{
		ID:       1,
		Name:     "NYC City Council",
		Location: "New York City",
		URL:      "https://council.nyc.gov/",
		Resolver: nyc.New(),
	},
}
