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
var congressBillPattern = regexp.MustCompile(`/bill/(\d+)th-congress/(house-bill|senate-bill|house-joint-resolution|senate-joint-resolution|house-concurrent-resolution|senate-concurrent-resolution|house-resolution|senate-resolution)/(\d+)`)

// SupportedDomains for House
func (h House) SupportedDomains() []string {
	return []string{"congress.gov"}
}

// SupportedDomains for Senate
func (s Senate) SupportedDomains() []string {
	return []string{"congress.gov"}
}

// Lookup finds a House bill from a URL
func (h House) Lookup(ctx context.Context, u *url.URL) (*legislature.Legislation, error) {
	if u.Hostname() != "www.congress.gov" {
		return nil, nil
	}

	matches := congressBillPattern.FindStringSubmatch(u.Path)
	if matches == nil || len(matches) != 4 {
		return nil, nil
	}

	congressNum, billTypeName, number := matches[1], matches[2], matches[3]

	// Only handle House bills for the House resolver
	if !strings.HasPrefix(billTypeName, "house-") {
		return nil, nil
	}

	billType := billTypeNameToCode(billTypeName)
	if billType == "" {
		return nil, fmt.Errorf("unknown bill type: %s", billTypeName)
	}

	log.WithContext(ctx).Infof("found congress.gov House URL %s", u.String())

	congress, err := strconv.Atoi(congressNum)
	if err != nil {
		return nil, err
	}

	bill, err := h.api.GetBill(ctx, congress, billType, number)
	if err != nil {
		return nil, err
	}

	session := Sessions.Find(congress*2 + 1787) // Convert congress number to year
	return bill.ToLegislation(h.body.ID, session)
}

// Lookup finds a Senate bill from a URL
func (s Senate) Lookup(ctx context.Context, u *url.URL) (*legislature.Legislation, error) {
	if u.Hostname() != "www.congress.gov" {
		return nil, nil
	}

	matches := congressBillPattern.FindStringSubmatch(u.Path)
	if matches == nil || len(matches) != 4 {
		return nil, nil
	}

	congressNum, billTypeName, number := matches[1], matches[2], matches[3]

	// Only handle Senate bills for the Senate resolver
	if !strings.HasPrefix(billTypeName, "senate-") {
		return nil, nil
	}

	billType := billTypeNameToCode(billTypeName)
	if billType == "" {
		return nil, fmt.Errorf("unknown bill type: %s", billTypeName)
	}

	log.WithContext(ctx).Infof("found congress.gov Senate URL %s", u.String())

	congress, err := strconv.Atoi(congressNum)
	if err != nil {
		return nil, err
	}

	bill, err := s.api.GetBill(ctx, congress, billType, number)
	if err != nil {
		return nil, err
	}

	session := Sessions.Find(congress*2 + 1787) // Convert congress number to year
	return bill.ToLegislation(s.body.ID, session)
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

	session := SessionForCongress(congress)
	return bill.ToLegislation(h.body.ID, session)
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

	session := SessionForCongress(congress)
	return bill.ToLegislation(s.body.ID, session)
}

// Link generates a URL for a House bill
func (h House) Link(l legislature.LegislationID) *url.URL {
	congress, billType, number, err := parseBillID(l)
	if err != nil {
		return nil
	}

	return &url.URL{
		Scheme: "https",
		Host:   "www.congress.gov",
		Path:   fmt.Sprintf("/bill/%dth-congress/%s/%s", congress, billTypeToName(billType), number),
	}
}

// Link generates a URL for a Senate bill
func (s Senate) Link(l legislature.LegislationID) *url.URL {
	congress, billType, number, err := parseBillID(l)
	if err != nil {
		return nil
	}

	return &url.URL{
		Scheme: "https",
		Host:   "www.congress.gov",
		Path:   fmt.Sprintf("/bill/%dth-congress/%s/%s", congress, billTypeToName(billType), number),
	}
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
	return nil, fmt.Errorf("Scorecard not implemented for House")
}

// Scorecard is not yet implemented for Congress
func (s Senate) Scorecard(ctx context.Context, items []legislature.Scorable) (*legislature.Scorecard, error) {
	return nil, fmt.Errorf("Scorecard not implemented for Senate")
}
