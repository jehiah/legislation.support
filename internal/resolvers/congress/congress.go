package congress

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"os"

	"github.com/jehiah/legislation.support/internal/legislature"
	log "github.com/sirupsen/logrus"
)

// https://www.congress.gov/bill/117th-congress/house-bill/8555?s=4&r=1
// https://www.congress.gov/bill/117th-congress/senate-bill/874

// https://api.congress.gov/
// https://github.com/LibraryOfCongress/api.congress.gov/
// limit/offset up to 250
// rate limit 5k/hr

// ENV CONGRESS_GOV_APIKEY

type House struct {
	body legislature.Body
	api  *CongressAPI
}

func (h House) Body() legislature.Body { return h.body }

func NewHouse(body legislature.Body, token string) *House {
	return &House{
		body: body,
		api:  NewAPI(token),
	}
}

type Senate struct {
	body legislature.Body
	api  *CongressAPI
}

func (s Senate) Body() legislature.Body { return s.body }

func NewSenate(body legislature.Body, token string) *Senate {
	return &Senate{
		body: body,
		api:  NewAPI(token),
	}
}

type CongressAPI struct {
	token string
}

func NewAPI(token string) *CongressAPI {
	if token == "" {
		token = os.Getenv("CONGRESS_GOV_APIKEY")
	}
	if token == "" {
		panic("missing api key")
	}
	return &CongressAPI{
		token: token,
	}
}

func (a CongressAPI) get(ctx context.Context, path string, params url.Values, v interface{}) error {
	if params == nil {
		params = url.Values{}
	}
	u := "https://api.congress.gov" + path
	fullURL := u
	if encoded := params.Encode(); encoded != "" {
		fullURL = fullURL + "?" + encoded
	}
	log.WithContext(ctx).WithField("congress_api", fullURL).Debug("CongressAPI.get")
	req, err := http.NewRequestWithContext(ctx, "GET", fullURL, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", a.token)
	req.Header.Set("User-Agent", "https://legislation.support/")
	req.Header.Set("Accept", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	dec := json.NewDecoder(resp.Body)
	dec.UseNumber()
	return dec.Decode(v)
}

type Pagination struct {
	Count int    `json:"count"`
	Next  string `json:"next"`
}

func nextRequest(p Pagination) (string, url.Values) {
	if p.Next == "" {
		return "", nil
	}
	u, err := url.Parse(p.Next)
	if err != nil {
		return "", nil
	}
	params := u.Query()
	if params == nil {
		params = url.Values{}
	}
	params.Del("format")
	return u.Path, params
}
