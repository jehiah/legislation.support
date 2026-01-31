package congress

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/jehiah/legislation.support/internal/legislature"
)

// https://api.congress.gov/

type BillResponse struct {
	Bill Bill `json:"bill"`
}

// Bill represents a bill from Congress.gov API
// https://github.com/LibraryOfCongress/api.congress.gov/blob/main/Documentation/BillEndpoint.md
type Bill struct {
	Number                               string      `json:"number"`
	Congress                             json.Number `json:"congress"`
	Type                                 string      `json:"type"`              // Possible values are "HR", "S", "HJRES", "SJRES", "HCONRES", "SCONRES", "HRES", and "SRES".
	OriginChamber                        string      `json:"originChamber"`     // House, Senate
	OriginChamberCode                    string      `json:"originChamberCode"` // H, S
	Title                                string      `json:"title"`
	IntroducedDate                       string      `json:"introducedDate"`
	UpdateDate                           string      `json:"updateDate"`
	UpdateDateIncludingText              string      `json:"updateDateIncludingText"`
	ConstitutionalAuthorityStatementText string      `json:"constitutionalAuthorityStatementText"`

	Sponsors   []BillSponsor `json:"sponsors"`
	Cosponsors struct {
		Count                             json.Number `json:"count"`
		CountIncludingWithdrawnCosponsors json.Number `json:"countIncludingWithdrawnCosponsors"`
		URL                               string      `json:"url"`
	} `json:"cosponsors"`

	Committees struct {
		Count json.Number `json:"count"`
		URL   string      `json:"url"`
	} `json:"committees"`

	Actions struct {
		Count json.Number  `json:"count"`
		URL   string       `json:"url"`
		Items []BillAction `json:"actions"`
	} `json:"actions"`

	Summaries struct {
		Count json.Number   `json:"count"`
		URL   string        `json:"url"`
		Items []BillSummary `json:"summaries"`
	} `json:"summaries"`

	Titles struct {
		Count json.Number `json:"count"`
		URL   string      `json:"url"`
		Items []BillTitle `json:"titles"`
	} `json:"titles"`

	LatestAction struct {
		ActionDate string `json:"actionDate"`
		Text       string `json:"text"`
	} `json:"latestAction"`

	Laws []struct {
		Number string `json:"number"`
		Type   string `json:"type"`
	} `json:"laws"`

	PolicyArea struct {
		Name string `json:"name"`
	} `json:"policyArea"`

	Subjects struct {
		Count json.Number `json:"count"`
		URL   string      `json:"url"`
	} `json:"subjects"`
}

type BillSponsor struct {
	BioguideID  string      `json:"bioguideId"`
	FullName    string      `json:"fullName"`
	FirstName   string      `json:"firstName"`
	LastName    string      `json:"lastName"`
	Party       string      `json:"party"`
	State       string      `json:"state"`
	District    json.Number `json:"district"`
	IsByRequest string      `json:"isByRequest"`
	URL         string      `json:"url"`
}

type BillAction struct {
	ActionDate   string `json:"actionDate"`
	ActionTime   string `json:"actionTime"`
	Text         string `json:"text,omitempty"`
	Type         string `json:"type"`
	ActionCode   string `json:"actionCode"`
	SourceSystem struct {
		Code int    `json:"code"`
		Name string `json:"name"`
	} `json:"sourceSystem"`
	Committees    []CommitteeAction `json:"committees"`
	RecordedVotes []RecordedVote    `json:"recordedVotes"`
}

type CommitteeAction struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	SystemCode string `json:"systemCode"`
}

type RecordedVote struct {
	Chamber       string `json:"chamber"`
	Congress      int    `json:"congress"`
	Date          string `json:"date"`
	RollNumber    int    `json:"rollNumber"`
	SessionNumber int    `json:"sessionNumber"`
	URL           string `json:"url"` // URL to XML vote record
}

type BillSummary struct {
	ActionDate  string `json:"actionDate"`
	ActionDesc  string `json:"actionDesc"`
	Text        string `json:"text"`
	UpdateDate  string `json:"updateDate"`
	VersionCode string `json:"versionCode"`
}

type BillTitle struct {
	Title               string `json:"title"`
	TitleType           string `json:"titleType"`
	TitleTypeCode       string `json:"titleTypeCode"`
	BillTextVersionName string `json:"billTextVersionName"`
	BillTextVersionCode string `json:"billTextVersionCode"`
}

// ID returns the LegislationID for this bill (e.g., "118-hr1234")
func (b Bill) ID() legislature.LegislationID {
	return legislature.LegislationID(fmt.Sprintf("%s-%s%s", b.Congress, b.Type, b.Number))
}

// ToLegislatureMember converts a BillSponsor to a legislature.Member
func (s BillSponsor) ToLegislatureMember() legislature.Member {
	return legislature.Member{
		Slug:      s.BioguideID,
		FullName:  s.FullName,
		ShortName: s.FirstName + " " + s.LastName,
		District:  normalizeDistrict(s.District),
		URL:       s.URL,
	}
}

// ToLegislation converts a Congress.gov Bill to a legislature.Legislation
func (b Bill) ToLegislation(bodyID legislature.BodyID, session legislature.Session) (*legislature.Legislation, error) {
	// Convert sponsors
	var sponsors []legislature.Member
	for _, s := range b.Sponsors {
		sponsors = append(sponsors, s.ToLegislatureMember())
	}

	// Get the primary title
	title := b.Title
	if title == "" && len(b.Titles.Items) > 0 {
		// Look for "Official Title as Introduced" or similar
		for _, t := range b.Titles.Items {
			if t.TitleTypeCode == "1" || t.TitleType == "Official Title as Introduced" {
				title = t.Title
				break
			}
		}
		// Fallback to first title
		if title == "" {
			title = b.Titles.Items[0].Title
		}
	}

	// Get summary
	var summary string
	if len(b.Summaries.Items) > 0 {
		summary = b.Summaries.Items[0].Text
	}

	// Parse dates
	introducedDate, _ := parseDate(b.IntroducedDate)
	lastModified, _ := parseDate(b.UpdateDate)

	// Determine bill type
	billType := legislature.BillType
	if b.Type == "HRES" || b.Type == "SRES" || b.Type == "HCONRES" || b.Type == "SCONRES" || b.Type == "HJRES" || b.Type == "SJRES" {
		billType = legislature.ResolutionType
	}

	// Build the URL
	congressNum := b.Congress.String()
	billURL := fmt.Sprintf("https://www.congress.gov/bill/%sth-congress/%s/%s", congressNum, billTypeToName(b.Type), b.Number)

	leg := &legislature.Legislation{
		Body:           bodyID,
		ID:             b.ID(),
		DisplayID:      formatDisplayID(b.Type, b.Number),
		Title:          title,
		Summary:        summary,
		Description:    "",
		URL:            billURL,
		Session:        session,
		Status:         b.LatestAction.Text,
		Type:           billType,
		Sponsors:       sponsors,
		IntroducedDate: introducedDate,
		LastModified:   lastModified,
	}

	return leg, nil
}

// formatDisplayID formats the display ID (e.g., "H.R. 1234", "S. 874")
func formatDisplayID(billType, number string) string {
	switch billType {
	case "HR":
		return fmt.Sprintf("H.R. %s", number)
	case "S":
		return fmt.Sprintf("S. %s", number)
	case "HJRES":
		return fmt.Sprintf("H.J.Res. %s", number)
	case "SJRES":
		return fmt.Sprintf("S.J.Res. %s", number)
	case "HCONRES":
		return fmt.Sprintf("H.Con.Res. %s", number)
	case "SCONRES":
		return fmt.Sprintf("S.Con.Res. %s", number)
	case "HRES":
		return fmt.Sprintf("H.Res. %s", number)
	case "SRES":
		return fmt.Sprintf("S.Res. %s", number)
	default:
		return fmt.Sprintf("%s %s", billType, number)
	}
}

// billTypeToName converts bill type code to full name for URL
func billTypeToName(billType string) string {
	switch billType {
	case "HR":
		return "house-bill"
	case "S":
		return "senate-bill"
	case "HJRES":
		return "house-joint-resolution"
	case "SJRES":
		return "senate-joint-resolution"
	case "HCONRES":
		return "house-concurrent-resolution"
	case "SCONRES":
		return "senate-concurrent-resolution"
	case "HRES":
		return "house-resolution"
	case "SRES":
		return "senate-resolution"
	default:
		return billType
	}
}

// parseDate attempts to parse a date string in YYYY-MM-DD format
func parseDate(dateStr string) (time.Time, error) {
	if dateStr == "" {
		return time.Time{}, nil
	}
	return time.Parse("2006-01-02", dateStr)
}
