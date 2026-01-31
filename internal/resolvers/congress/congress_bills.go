package congress

import (
	"context"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/jehiah/legislation.support/internal/legislature"
)

// GetBill fetches a bill from the Congress.gov API
// billType can be: hr, s, hjres, sjres, hconres, sconres, hres, sres
// Example: GetBill(ctx, 118, "hr", "1234")
func (a *CongressAPI) GetBill(ctx context.Context, congress int, billType, number string) (*Bill, error) {
	path := fmt.Sprintf("/v3/bill/%d/%s/%s", congress, billType, number)
	params := url.Values{"format": []string{"json"}}
	var resp BillResponse
	if err := a.get(ctx, path, params, &resp); err != nil {
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
	path := fmt.Sprintf("/v3/bill/%d/%s/%s/cosponsors", congress, billType, number)
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
	return allCosponsors, nil
}

// GetActions fetches the full list of actions for a bill
func (a *CongressAPI) GetActions(ctx context.Context, congress int, billType, number string) ([]BillAction, error) {
	path := fmt.Sprintf("/v3/bill/%d/%s/%s/actions", congress, billType, number)
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
	return allActions, nil
}
