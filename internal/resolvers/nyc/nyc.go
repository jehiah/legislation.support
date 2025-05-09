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

var introPattern = regexp.MustCompile("/(res-)?[0-9]{1,4}-20[12][0-9]$")

func fileToLegislationID(file string) legislature.LegislationID {
	fileType, fileNo, _ := strings.Cut(file, " ")
	switch fileType {
	case "Int":
		return legislature.LegislationID(fileNo)
	case "Res":
		return legislature.LegislationID("res-" + fileNo)
	default:
		return ""
	}
}

func (n NYC) SupportedDomains() []string {
	return []string{"legistar.council.nyc.gov", "intro.nyc"}
}

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
		Path:   "/" + string(id),
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
		Path:   "/" + string(l.ID),
	}
	return n.IntroJSON(ctx, u.String())
}
func (n NYC) Link(l legislature.LegislationID) *url.URL {
	return &url.URL{
		Scheme: "https",
		Host:   "intro.nyc",
		Path:   "/" + string(l),
	}
}

// 1234-2020 => 1234-2020
// res-1234-2020 => Res 1234-2020
func (n NYC) DisplayID(l legislature.LegislationID) string {
	if strings.HasPrefix(string(l), "res-") {
		return "Res " + strings.TrimPrefix(string(l), "res-")
	}
	return string(l)
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
	fileType, fileNo, _ := strings.Cut(d.File, " ")

	path := fileNo
	legType := legislature.BillType
	switch fileType {
	case "Int":
	case "Res":
		path = "res-" + fileNo
		legType = legislature.ResolutionType
	default:
		log.Printf("unknown file type %q %q", fileType, d.File)
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
		ID:             fileToLegislationID(d.File),
		DisplayID:      d.File,
		Title:          d.Name,
		Summary:        d.Title,
		Description:    d.Summary,
		IntroducedDate: d.IntroDate,
		Session:        Sessions.Find(d.IntroDate.Year()),
		Status:         d.StatusName,
		Type:           legType,
		Sponsors:       sponsors,
		LastModified:   d.LastModified,
		URL:            "https://intro.nyc/" + path,
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
		return fmt.Errorf("got http %d from %q", resp.StatusCode, r.URL.String())
	}
	return json.NewDecoder(resp.Body).Decode(&v)
}

func (n NYC) IntroJSON(ctx context.Context, u string) (*db.Legislation, error) {
	var d db.Legislation
	err := n.get(ctx, u+".json", &d)
	if err != nil {
		return nil, err
	}
	if d.File == "" {
		return nil, nil
	}
	return &d, err
}

type resubmit struct {
	Resubmitted []resubmitMapping
}
type resubmitMapping struct {
	FromFile string
	ToFile   string
}

func (n NYC) Resubmit(ctx context.Context, year int) (legislature.ResubmitMapping, error) {
	u := fmt.Sprintf("https://intro.nyc/data/resubmit_%d.json", year)
	var r resubmit
	err := n.get(ctx, u, &r)
	if err != nil {
		return nil, err
	}
	m := make(legislature.ResubmitMapping)
	for _, v := range r.Resubmitted {
		m[legislature.GlobalID{
			BodyID:        n.body.ID,
			LegislationID: fileToLegislationID(v.FromFile),
		}] = legislature.GlobalID{
			BodyID:        n.body.ID,
			LegislationID: fileToLegislationID(v.ToFile),
		}
	}
	return m, nil
}
