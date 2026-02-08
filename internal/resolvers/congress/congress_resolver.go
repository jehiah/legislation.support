package congress

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"github.com/jehiah/legislation.support/internal/legislature"
	log "github.com/sirupsen/logrus"
)

// SupportedDomains for House
func (h House) SupportedDomains() []string {
	return []string{"www.congress.gov", "congress.gov"}
}

// SupportedDomains for Senate
func (s Senate) SupportedDomains() []string {
	return []string{"www.congress.gov", "congress.gov"}
}

// Lookup finds a House bill from a URL
func (h House) Lookup(ctx context.Context, u *url.URL) (*legislature.Legislation, error) {
	bill, err := h.api.Lookup(ctx, u)
	if err != nil {
		return nil, err
	}
	if bill.OriginChamber != "House" {
		return nil, fmt.Errorf("bill %s has OriginChamber:%q", bill.Number, bill.OriginChamber)
	}
	return bill.ToLegislation(h.body.ID)
}

// Lookup finds a Senate bill from a URL
func (s Senate) Lookup(ctx context.Context, u *url.URL) (*legislature.Legislation, error) {
	bill, err := s.api.Lookup(ctx, u)
	if err != nil {
		return nil, err
	}
	if bill.OriginChamber != "Senate" {
		return nil, fmt.Errorf("bill %s has OriginChamber:%q", bill.Number, bill.OriginChamber)
	}
	return bill.ToLegislation(s.body.ID)
}

// billTypeNameToCode converts URL bill type name to API code
func billTypeNameToCode(name string) string {
	switch name {
	case "house-bill":
		return "hr"
	case "senate-bill":
		return "s"
	case "house-joint-resolution":
		return "hjres"
	case "senate-joint-resolution":
		return "sjres"
	case "house-concurrent-resolution":
		return "hconres"
	case "senate-concurrent-resolution":
		return "sconres"
	case "house-resolution":
		return "hres"
	case "senate-resolution":
		return "sres"
	default:
		return ""
	}
}

// Refresh fetches updated data for a House bill
func (h House) Refresh(ctx context.Context, billID legislature.LegislationID) (*legislature.Legislation, error) {
	congress, billType, number, err := parseBillID(billID)
	if err != nil {
		return nil, err
	}

	// Validate it's a House bill
	if !strings.HasPrefix(billType, "h") {
		return nil, fmt.Errorf("House resolver cannot refresh Senate bill: %q", billID)
	}

	bill, err := h.api.GetBill(ctx, congress, billType, number)
	if err != nil {
		return nil, err
	}

	return bill.ToLegislation(h.body.ID)
}

// Refresh fetches updated data for a Senate bill
func (s Senate) Refresh(ctx context.Context, billID legislature.LegislationID) (*legislature.Legislation, error) {
	congress, billType, number, err := parseBillID(billID)
	if err != nil {
		return nil, err
	}

	// Validate it's a Senate bill
	if !strings.HasPrefix(billType, "s") {
		return nil, fmt.Errorf("Senate resolver cannot refresh House bill: %q", billID)
	}

	bill, err := s.api.GetBill(ctx, congress, billType, number)
	if err != nil {
		return nil, err
	}

	return bill.ToLegislation(s.body.ID)
}

func Link(l legislature.LegislationID) *url.URL {
	congress, billType, number, err := parseBillID(l)
	if err != nil {
		log.Errorf("congress.Link: unable to parse bill ID %q: %v", l, err)
	}
	return &url.URL{
		Scheme: "https",
		Host:   "www.congress.gov",
		Path:   fmt.Sprintf("/bill/%dth-congress/%s/%s", congress, billTypeToName(billType), number),
	}
}

// Link generates a URL for a House bill
func (h House) Link(l legislature.LegislationID) *url.URL {
	return Link(l)
}

// Link generates a URL for a Senate bill
func (s Senate) Link(l legislature.LegislationID) *url.URL {
	return Link(l)
}

// DisplayID returns a formatted display ID (e.g., "H.R. 1234")
func (h House) DisplayID(l legislature.LegislationID) string {
	_, billType, number, err := parseBillID(l)
	if err != nil {
		return string(l)
	}
	return formatDisplayID(billType, number)
}

// DisplayID returns a formatted display ID (e.g., "S. 874")
func (s Senate) DisplayID(l legislature.LegislationID) string {
	_, billType, number, err := parseBillID(l)
	if err != nil {
		return string(l)
	}
	return formatDisplayID(billType, number)
}

// Scorecard is not yet implemented for Congress
func (h House) Scorecard(ctx context.Context, items []legislature.Scorable) (*legislature.Scorecard, error) {
	return scorecardForCongress(ctx, h.body, h.api, ChamberHouse, items)
}

// Scorecard is not yet implemented for Congress
func (s Senate) Scorecard(ctx context.Context, items []legislature.Scorable) (*legislature.Scorecard, error) {
	return scorecardForCongress(ctx, s.body, s.api, ChamberSenate, items)
}
