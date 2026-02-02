package congress

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/jehiah/legislation.support/internal/legislature"
	log "github.com/sirupsen/logrus"
)

// Regex patterns for congress.gov URLs
// Examples:
// https://www.congress.gov/bill/118th-congress/house-bill/1234
// https://www.congress.gov/bill/118th-congress/senate-bill/874
// https://www.congress.gov/bill/117th-congress/house-bill/8555?s=4&r=1
// https://www.congress.gov/bill/117th-congress/senate-bill/874
var congressBillPattern = regexp.MustCompile(`/bill/(\d+)th-congress/(house-bill|senate-bill|house-joint-resolution|senate-joint-resolution|house-concurrent-resolution|senate-concurrent-resolution|house-resolution|senate-resolution)/(\d+)`)

var congressHosts = map[string]bool{
	"www.congress.gov": true,
	"congress.gov":     true,
}

func (a *CongressAPI) Lookup(ctx context.Context, u *url.URL) (*Bill, error) {
	if !congressHosts[u.Hostname()] {
		return nil, nil
	}

	matches := congressBillPattern.FindStringSubmatch(u.Path)
	if matches == nil || len(matches) != 4 {
		return nil, nil
	}

	congressNumStr, billTypeName, number := matches[1], matches[2], matches[3]
	congressNum, err := strconv.Atoi(strings.TrimSuffix(congressNumStr, "th-congress"))
	if err != nil {
		return nil, err
	}
	return a.GetBill(ctx, congressNum, billTypeNameToCode(billTypeName), number)
}

// GetBill fetches a bill from the Congress.gov API
// billType can be: hr, s, hjres, sjres, hconres, sconres, hres, sres
// Example: GetBill(ctx, 118, "hr", "1234")
func (a *CongressAPI) GetBill(ctx context.Context, congress int, billType, number string) (*Bill, error) {
	path := fmt.Sprintf("/v3/bill/%d/%s/%s", congress, billType, number)
	params := url.Values{"format": []string{"json"}}
	var resp BillResponse
	err := a.get(ctx, path, params, &resp)
	if err != nil {
		return nil, err
	}
	resp.Bill.Cosponsors.Items, err = a.GetCosponsors(ctx, congress, billType, number)
	if err != nil {
		return nil, err
	}
	return &resp.Bill, nil
}

// GetBillByID parses a LegislationID (e.g., "118-hr1234") and fetches the bill
func (a *CongressAPI) GetBillByID(ctx context.Context, id legislature.LegislationID) (*Bill, error) {
	congress, billType, number, err := parseBillID(id)
	if err != nil {
		return nil, err
	}
	return a.GetBill(ctx, congress, billType, number)
}

var BillTypes = []string{
	"HJRES",
	"SJRES",
	"HCONRES",
	"SCONRES",
	"HRES",
	"SRES",
	"HR",
	"S",
}

// parseBillID parses a LegislationID like "118-hr1234" into its components
func parseBillID(id legislature.LegislationID) (congress int, billType, number string, err error) {
	// Format: "118-hr1234"
	congressStr, rest, ok := strings.Cut(string(id), "-")
	if !ok {
		return 0, "", "", fmt.Errorf("invalid bill ID format: %s", id)
	}

	// Parse congress number
	congress, err = strconv.Atoi(congressStr)
	if err != nil {
		return 0, "", "", fmt.Errorf("invalid congress number in bill ID: %s", id)
	}

	// Extract bill type and number from rest (e.g., "hr1234")
	// Bill types: hr, s, hjres, sjres, hconres, sconres, hres, sres
	for _, bt := range BillTypes {
		if strings.HasPrefix(rest, bt) {
			billType = bt
			number = strings.TrimPrefix(rest, bt)
			return
		}
	}

	return 0, "", "", fmt.Errorf("invalid bill type in bill ID: %s", id)
}

// GetCosponsors fetches the full list of cosponsors for a bill
func (a *CongressAPI) GetCosponsors(ctx context.Context, congress int, billType, number string) ([]BillSponsor, error) {
	path := fmt.Sprintf("/v3/bill/%d/%s/%s/cosponsors", congress, strings.ToLower(billType), number)
	params := url.Values{
		"format": []string{"json"},
		"limit":  []string{"250"},
	}

	var allCosponsors []BillSponsor
	for {
		var resp struct {
			Cosponsors []BillSponsor `json:"cosponsors"`
			Pagination Pagination    `json:"pagination"`
		}
		if err := a.get(ctx, path, params, &resp); err != nil {
			return nil, err
		}
		allCosponsors = append(allCosponsors, resp.Cosponsors...)

		path, params = nextRequest(resp.Pagination)
		if path == "" {
			break
		}
	}
	log.Infof("Fetched %d cosponsors for %d %s %s", len(allCosponsors), congress, billType, number)
	return allCosponsors, nil
}

// GetActions fetches the full list of actions for a bill
func (a *CongressAPI) GetActions(ctx context.Context, congress int, billType, number string) ([]BillAction, error) {
	path := fmt.Sprintf("/v3/bill/%d/%s/%s/actions", congress, strings.ToLower(billType), number)
	params := url.Values{
		"format": []string{"json"},
		"limit":  []string{"250"},
	}

	var allActions []BillAction
	for {
		var resp struct {
			Actions    []BillAction `json:"actions"`
			Pagination Pagination   `json:"pagination"`
		}
		if err := a.get(ctx, path, params, &resp); err != nil {
			return nil, err
		}
		allActions = append(allActions, resp.Actions...)

		path, params = nextRequest(resp.Pagination)
		if path == "" {
			break
		}
	}
	log.Infof("Fetched %d actions for %d %s %s", len(allActions), congress, billType, number)
	return allActions, nil
}
