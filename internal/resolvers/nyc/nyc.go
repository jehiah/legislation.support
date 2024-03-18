package nyc

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/jehiah/legislation.support/internal/legislature"
	"github.com/jehiah/legislator/db"
	log "github.com/sirupsen/logrus"
)

var Sessions = legislature.Sessions{
	{2024, 2025},
	{2022, 2023},
	{2018, 2021},
	{2014, 2017},
	{2010, 2013},
	{2006, 2009},
	{2004, 2005},
	{2002, 2003},
	{1998, 2001},
}

// var CurrentSession = Sessions.Current()

type NYC struct {
	body legislature.Body
}

func New(b legislature.Body) *NYC {
	return &NYC{body: b}
}

func (n NYC) Body() legislature.Body { return n.body }

var introPattern = regexp.MustCompile("/[0-9]{1,4}-20[12][0-9]$")

func (n NYC) Lookup(ctx context.Context, u *url.URL) (*legislature.Legislation, error) {
	switch u.Hostname() {
	case "legistar.council.nyc.gov", "nyc.legistar.com":
		if u.Path != "/LegislationDetail.aspx" {
			return nil, nil
		}
		u, err := n.LookupLegistarLegislationDetail(ctx, u)
		if err != nil {
			return nil, err
		}
		if u != nil {
			d, err := n.IntroJSON(ctx, u.String())
			if err != nil {
				return nil, err
			}
			return n.NewLegislation(d), nil
		}
	case "intro.nyc":
		if !introPattern.MatchString(u.Path) {
			return nil, nil
		}
		d, err := n.IntroJSON(ctx, u.String())
		if err != nil {
			return nil, err
		}
		return n.NewLegislation(d), nil
	}
	return nil, nil
}

func (n NYC) Refresh(ctx context.Context, id legislature.LegislationID) (*legislature.Legislation, error) {
	u := &url.URL{
		Scheme: "https",
		Host:   "intro.nyc",
		Path:   "/" + n.DisplayID(id),
	}
	d, err := n.IntroJSON(ctx, u.String())
	if err != nil {
		return nil, err
	}
	return n.NewLegislation(d), nil
}

func (n NYC) Raw(ctx context.Context, l *legislature.Legislation) (*db.Legislation, error) {
	u := &url.URL{
		Scheme: "https",
		Host:   "intro.nyc",
		Path:   "/" + n.DisplayID(l.ID),
	}
	return n.IntroJSON(ctx, u.String())
}
func (n NYC) Link(l legislature.LegislationID) *url.URL {
	return &url.URL{
		Scheme: "https",
		Host:   "intro.nyc",
		Path:   "/" + n.DisplayID(l),
	}
}
func (n NYC) DisplayID(l legislature.LegislationID) string {
	return strings.TrimPrefix(string(l), "Int ")
}

func (n NYC) ActivePeople(ctx context.Context) ([]db.Person, error) {
	u := &url.URL{
		Scheme: "https",
		Host:   "intro.nyc",
		Path:   "/data/people_active.json",
	}
	var people []db.Person
	err := n.get(ctx, u.String(), &people)
	return people, err
}

func (n NYC) AllPeople(ctx context.Context) ([]db.Person, error) {
	u := &url.URL{
		Scheme: "https",
		Host:   "intro.nyc",
		Path:   "/data/people_all.json",
	}
	var people []db.Person
	err := n.get(ctx, u.String(), &people)
	return people, err
}

type PersonMetadata struct {
	ID       int
	District int
}

func (n NYC) PersonMetadata(ctx context.Context) ([]PersonMetadata, error) {
	u := &url.URL{
		Scheme: "https",
		Host:   "intro.nyc",
		Path:   "/data/people_metadata.json",
	}
	var md []PersonMetadata
	err := n.get(ctx, u.String(), &md)
	return md, err
}

func (n NYC) NewLegislation(d *db.Legislation) *legislature.Legislation {
	if d == nil {
		return nil
	}

	sponsors := make([]legislature.Member, 0, len(d.Sponsors))
	for _, p := range d.Sponsors {
		if p.ID == 0 {
			continue // borough president, etc
		}
		sponsors = append(sponsors, legislature.Member{
			NumericID: p.ID,
			FullName:  strings.TrimSpace(p.FullName),
			URL:       "https://intro.nyc/councilmembers/" + p.Slug,
			Slug:      p.Slug,
		})
	}

	return &legislature.Legislation{
		Body:           n.body.ID,
		ID:             legislature.LegislationID(strings.TrimPrefix(d.File, "Int ")),
		DisplayID:      d.File,
		Title:          d.Name,
		Summary:        d.Title,
		Description:    d.Summary,
		IntroducedDate: d.IntroDate,
		Session:        Sessions.Find(d.IntroDate.Year()),
		Status:         d.StatusName,
		Sponsors:       sponsors,
		LastModified:   d.LastModified,
		URL:            "https://intro.nyc/" + strings.TrimPrefix(d.File, "Int "),
	}
}

func (n NYC) get(ctx context.Context, u string, v interface{}) error {
	r, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		log.Printf("GET %s %s", r.URL.String(), err)
		return err
	}
	defer resp.Body.Close()
	log.Printf("%d GET %s", resp.StatusCode, r.URL.String())
	if resp.StatusCode == 404 {
		v = nil
		return nil
	}
	if resp.StatusCode != 200 {
		return fmt.Errorf("got http %d", resp.StatusCode)
	}
	return json.NewDecoder(resp.Body).Decode(&v)
}

func (n NYC) IntroJSON(ctx context.Context, u string) (*db.Legislation, error) {
	var d db.Legislation
	err := n.get(ctx, u+".json", &d)
	if err != nil {
		return nil, err
	}
	return &d, err
}
