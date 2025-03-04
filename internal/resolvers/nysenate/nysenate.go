package nysenate

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jehiah/legislation.support/internal/legislature"
	log "github.com/sirupsen/logrus"
)

var Sessions = legislature.Sessions{
	{2025, 2026},
	{2023, 2024},
	{2021, 2022},
	{2019, 2020},
	{2017, 2018},
	{2015, 2016},
	{2013, 2014},
	{2011, 2012},
	{2009, 2010},
	{2007, 2008},
}

type chamber string

const senateChamber chamber = "senate"
const assemblyChamber chamber = "assembly"

type NYSenate struct {
	body legislature.Body
	api  *NYSenateAPI
}

func (n NYSenate) Body() legislature.Body { return n.body }

func NewNYSenate(body legislature.Body, token string) *NYSenate {
	return &NYSenate{
		body: body,
		api:  NewAPI(token),
	}
}

type NYAssembly struct {
	body legislature.Body
	api  *NYSenateAPI
}

func (n NYAssembly) Body() legislature.Body { return n.body }

func NewNYAssembly(body legislature.Body, token string) *NYAssembly {
	return &NYAssembly{
		body: body,
		api:  NewAPI(token),
	}
}

// LegislationSort is a custom sort function for NYSenate legislation
// sorting upper chamber bills first, then lower chamber
func LegislationSort(a, b *legislature.Legislation) bool {
	aID, bID := a.ID, b.ID
	// prefer to sort with the ID for the Upper House
	if a.SameAs != "" && aID[5] == 'A' {
		aID = a.SameAs
	}
	if b.SameAs != "" && bID[5] == 'A' {
		bID = b.SameAs
	}
	switch {
	// case a.Body != b.Body:
	// 	return a.Body < b.Body
	case a.Session != b.Session:
		return a.Session.StartYear < b.Session.StartYear
	case aID[5] != bID[5]:
		return aID[5] == 'S'
	default:
		aa, _ := strconv.Atoi(string(aID)[6:]) // i.e 2020-S1234
		bb, _ := strconv.Atoi(string(bID)[6:])
		return aa < bb
	}
}

var nysenatePattern = regexp.MustCompile("/legislation/bills/((199|200|201|202)[0-9])/((S|s)[0-9]+)([A-F]|/|/amendment.*)?$")
var nyAssemblyPattern = regexp.MustCompile("/legislation/bills/((199|200|201|202)[0-9])/((A|a)[0-9]+)([A-F]|/|/amendment.*)?$")

func (a NYSenate) SupportedDomains() []string {
	return []string{"nysenate.gov"}
}

func (a NYSenate) Lookup(ctx context.Context, u *url.URL) (*legislature.Legislation, error) {
	switch u.Hostname() {
	case "www.nysenate.gov":
	default:
		return nil, nil
	}
	p := nysenatePattern.FindStringSubmatch(u.Path)
	if p == nil {
		return nil, nil
	}
	if len(p) != 6 {
		log.Warnf("NYSenate no match %#v %s", p, u.String())
		return nil, nil
	}
	log.Infof("found nysenate URL %s", u.String())
	session, printNo := p[1], p[3]
	bill, err := a.api.GetBill(ctx, session, printNo)
	if err != nil {
		return nil, err
	}
	return bill.Legislation(a.body.ID), nil
}

func (a NYAssembly) SupportedDomains() []string {
	return []string{"assembly.state.ny.us"}
}

func (a NYAssembly) Lookup(ctx context.Context, u *url.URL) (*legislature.Legislation, error) {
	var session, printNo string
	switch u.Hostname() {
	case "www.nysenate.gov":
		p := nyAssemblyPattern.FindStringSubmatch(u.Path)
		if len(p) != 6 {
			log.Debugf("NYAssembly no match %#v %s", p, u.String())
			return nil, nil
		}
		log.Infof("found nysenate URL %s", u.String())
		session, printNo = p[1], p[3]
	case "assembly.state.ny.us", "nyassembly.gov":
		if u.Path != "/leg/" {
			return nil, nil
		}
		session, printNo = u.Query().Get("term"), u.Query().Get("bn")
	// TODO:
	// https://legiscan.com/NY/bill/S0{number}/{year}
	// i.e. https://legiscan.com/NY/bill/S01046/2021.json
	default:
		return nil, nil
	}
	bill, err := a.api.GetBill(ctx, session, printNo)
	if err != nil {
		return nil, err
	}
	if strings.HasPrefix(printNo, "S") {
		return bill.Legislation(a.body.Bicameral), nil
	}
	return bill.Legislation(a.body.ID), nil
}

func (a NYSenate) Refresh(ctx context.Context, billID legislature.LegislationID) (*legislature.Legislation, error) {
	session, printNo, _ := strings.Cut(string(billID), "-")
	if !strings.HasPrefix(printNo, "S") {
		return nil, fmt.Errorf("invalid %q", billID)
	}
	bill, err := a.api.GetBill(ctx, session, printNo)
	if err != nil {
		return nil, err
	}
	return bill.Legislation(a.body.ID), nil
}

func (a NYAssembly) Refresh(ctx context.Context, billID legislature.LegislationID) (*legislature.Legislation, error) {
	session, printNo, _ := strings.Cut(string(billID), "-")
	if !strings.HasPrefix(printNo, "A") {
		return nil, fmt.Errorf("invalid %q", billID)
	}
	bill, err := a.api.GetBill(ctx, session, printNo)
	if err != nil {
		return nil, err
	}
	return bill.Legislation(a.body.ID), nil
}

func (bill *Bill) Legislation(body legislature.BodyID) *legislature.Legislation {
	if bill == nil {
		return nil
	}
	t, _ := time.Parse("2006-01-02T15:04:05", bill.PublishedDateTime)
	session := Sessions.Find(bill.Session)
	if session == (legislature.Session{}) {
		log.Errorf("unable to find session %v", bill.Session)
		return nil
	}
	chamber := assemblyChamber
	if body != "ny-assembly" {
		chamber = senateChamber
	}
	var sponsors []legislature.Member
	for _, m := range bill.GetSponsors() {
		sponsors = append(sponsors, legislature.Member{
			NumericID: m.MemberID,
			FullName:  m.FullName,
			ShortName: m.ShortName,
			URL:       memberURL(m.FullName, chamber),
			// District:  fmt.Sprintf("%d", mmm.DistrictCode),
			// URL: fmt.Sprintf("https://www.nysenate.gov/senators/%d", m.MemberID)
		})
	}

	return &legislature.Legislation{
		ID:             bill.ID(),
		Body:           body,
		DisplayID:      bill.BasePrintNo,
		Title:          bill.Title,
		Summary:        bill.Summary,
		IntroducedDate: t,
		Session:        session,
		SameAs:         bill.GetSameAs(),
		SubstitutedBy:  bill.GetSubstitutedBy(),
		Sponsors:       sponsors,
		URL:            fmt.Sprintf("https://www.nysenate.gov/legislation/bills/%d/%s", bill.Session, bill.BasePrintNo),
	}
}
func (NYSenateAPI) Link(l legislature.LegislationID) *url.URL {
	session, printNo := splitLegislationID(l)
	return &url.URL{
		Scheme: "https",
		Host:   "www.nysenate.gov",
		Path:   fmt.Sprintf("/legislation/bills/%s/%s", session, printNo),
	}
}
func (a NYAssembly) Link(l legislature.LegislationID) *url.URL { return a.api.Link(l) }
func (a NYSenate) Link(l legislature.LegislationID) *url.URL   { return a.api.Link(l) }

func (NYSenateAPI) DisplayID(l legislature.LegislationID) string {
	_, printNo := splitLegislationID(l)
	return printNo
}
func (a NYAssembly) DisplayID(l legislature.LegislationID) string { return a.api.DisplayID(l) }
func (a NYSenate) DisplayID(l legislature.LegislationID) string   { return a.api.DisplayID(l) }

func NewAPI(token string) *NYSenateAPI {
	if token == "" {
		panic("missing token")
	}
	return &NYSenateAPI{
		token: token,
	}
}
