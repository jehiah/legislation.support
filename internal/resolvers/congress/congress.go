package congress

import "github.com/jehiah/legislation.support/internal/legislature"

// https://www.congress.gov/bill/117th-congress/house-bill/8555?s=4&r=1
// https://www.congress.gov/bill/117th-congress/senate-bill/874

// https://api.congress.gov/

// ENV CONGRESS_GOV_APIKEY

// curl -H "X-API-Key: $PRO_PUBLICA_CONGRESS_API_KEY" "https://api.propublica.org/congress/v1/117/bills/hr8555.json"

type Congress struct {
	body legislature.Body
	api  *CongressAPI
}
type CongressAPI struct {
	token string
}

func NewAPI(token string) *CongressAPI {
	if token == "" {
		panic("missing api key")
	}
	return &CongressAPI{
		token: token,
	}
}
