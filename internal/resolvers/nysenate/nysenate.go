package nysenate

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"github.com/jehiah/legislation.support/internal/legislature"
	log "github.com/sirupsen/logrus"
)

var Sessions = legislature.Sessions{
	{2024, 2025},
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

func LegislationSort(a, b *legislature.Legislation) bool {
	switch {
	case a.Body != b.Body:
		return a.Body < b.Body
	case a.Session != b.Session:
		return a.Session.StartYear < b.Session.StartYear
	case a.ID[5] != b.ID[5]:
		return a.ID[5] < b.ID[5]
	default:
		aa, _ := strconv.Atoi(string(a.ID)[6:]) // i.e 2020-S1234
		bb, _ := strconv.Atoi(string(b.ID)[6:])
		return aa < bb
	}
}

var nysenatePattern = regexp.MustCompile("/legislation/bills/((199|200|201|202)[0-9])/((S|s)[0-9]+)(/amendment.*)?$")
var nyAssemblyPattern = regexp.MustCompile("/legislation/bills/((199|200|201|202)[0-9])/((A|a)[0-9]+)(/amendment.*)?$")

func (a NYSenate) Lookup(ctx context.Context, u *url.URL) (*legislature.Legislation, error) {
	switch u.Hostname() {
	case "www.nysenate.gov":
	default:
		return nil, nil
	}
	p := nysenatePattern.FindStringSubmatch(u.Path)
	if len(p) != 6 {
		log.Printf("no match %#v %s", p, u.String())
		return nil, nil
	}
	log.Printf("found nysenate URL %s", u.String())
	session, printNo := p[1], p[3]
	bill, err := a.api.GetBill(ctx, session, printNo)
	if err != nil {
		return nil, err
	}
	return bill.Legislation(a.body.ID), nil
}

func (a NYAssembly) Lookup(ctx context.Context, u *url.URL) (*legislature.Legislation, error) {
	var session, printNo string
	switch u.Hostname() {
	case "www.nysenate.gov":
		p := nyAssemblyPattern.FindStringSubmatch(u.Path)
		if len(p) != 6 {
			log.Infof("no match %#v %s", p, u.String())
			return nil, nil
		}
		log.Infof("found nysenate URL %s", u.String())
		session, printNo = p[1], p[3]
	case "assembly.state.ny.us":
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
	return &legislature.Legislation{
		ID:             legislature.LegislationID(fmt.Sprintf("%d-%s", bill.Session, bill.BasePrintNo)),
		Body:           body,
		DisplayID:      bill.BasePrintNo,
		Title:          bill.Title,
		Summary:        bill.Summary,
		IntroducedDate: t,
		Session:        session,
		URL:            fmt.Sprintf("https://www.nysenate.gov/legislation/bills/%d/%s", bill.Session, bill.BasePrintNo),
	}
}

func NewAPI(token string) *NYSenateAPI {
	if token == "" {
		panic("missing token")
	}
	return &NYSenateAPI{
		token: token,
	}
}
