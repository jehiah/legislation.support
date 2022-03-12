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

type NYC struct {
	Body legislature.Body
}

func New(b legislature.Body) *NYC {
	return &NYC{Body: b}
}

var introPattern = regexp.MustCompile("/[0-9]{1,4}-20[12][0-9]$")

func (n NYC) Lookup(ctx context.Context, u *url.URL) (*legislature.Legislation, error) {
	switch u.Hostname() {
	case "legistar.council.nyc.gov":
		if u.Path != "/LegislationDetail.aspx" {
			return nil, legislature.ErrNotFound
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

func (n NYC) NewLegislation(d *db.Legislation) *legislature.Legislation {
	if d == nil {
		return nil
	}
	return &legislature.Legislation{
		Body:        n.Body.ID,
		ID:          legislature.LegislationID(strings.TrimPrefix(d.File, "Int ")),
		DisplayID:   d.File,
		Title:       d.Name,
		Summary:     d.Title,
		Description: d.Summary,
		URL:         "https://intro.nyc/" + strings.TrimPrefix(d.File, "Int "),
		// TODO Session
	}
}

func (n NYC) IntroJSON(ctx context.Context, u string) (*db.Legislation, error) {
	r, err := http.NewRequestWithContext(ctx, "GET", u+".json", nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(r)
	if err != nil {
		log.Printf("GET %s %s", r.URL.String(), err)
		return nil, err
	}
	defer resp.Body.Close()
	log.Printf("%d GET %s", resp.StatusCode, r.URL.String())
	if resp.StatusCode == 404 {
		return nil, nil
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("got http %d", resp.StatusCode)
	}
	var d db.Legislation
	err = json.NewDecoder(resp.Body).Decode(&d)
	return &d, err
}
