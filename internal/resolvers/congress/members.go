package congress

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/jehiah/legislation.support/internal/legislature"
)

const ChamberHouse = "House"
const ChamberSenate = "Senate"

func (h *House) Members(ctx context.Context, session legislature.Session) ([]legislature.Member, error) {
	members, err := h.api.Members(ctx, session)
	if err != nil {
		return nil, err
	}
	var result []legislature.Member
	for _, m := range members {
		var match bool
		for _, term := range m.Term.Items {
			if term.ContainsSession(session) && strings.HasPrefix(term.Chamber, ChamberHouse) {
				match = true
				break
			}
		}
		if !match {
			continue
		}
		result = append(result, m.ToLegislatureMember())
	}
	return result, nil
}

func (s *Senate) Members(ctx context.Context, session legislature.Session) ([]legislature.Member, error) {
	members, err := s.api.Members(ctx, session)
	if err != nil {
		return nil, err
	}
	var result []legislature.Member
	for _, m := range members {
		var match bool
		for _, term := range m.Term.Items {
			if term.ContainsSession(session) && strings.HasPrefix(term.Chamber, ChamberSenate) {
				match = true
				break
			}
		}
		if !match {
			continue
		}
		result = append(result, m.ToLegislatureMember())
	}
	return result, nil
}

func (c *CongressAPI) Members(ctx context.Context, session legislature.Session) ([]Member, error) {
	path := fmt.Sprintf("/v3/member/congress/%d", congressNumber(session))
	params := url.Values{"limit": []string{"250"}}
	var members []Member
	for {
		var resp memberListResponse
		if err := c.get(ctx, path, params, &resp); err != nil {
			return nil, err
		}
		for _, m := range resp.Members {
			members = append(members, m)
		}
		path, params = nextRequest(resp.Pagination)
		if path == "" {
			break
		}
	}
	return members, nil
}

type memberListResponse struct {
	Members    []Member   `json:"members"`
	Pagination Pagination `json:"pagination"`
}

type Member struct {
	BioguideID string      `json:"bioguideId"`
	Name       string      `json:"name"`
	District   json.Number `json:"district"`
	PartyName  string      `json:"partyName"` // "Democratic", ...
	State      string      `json:"state"`     // "Rhode Island",
	URL        string      `json:"url"`
	Term       struct {
		Items []Term `json:"item"`
	} `json:"terms"`
}

type Term struct {
	StartYear int    `json:"startYear"`
	EndYear   int    `json:"endYear,omitempty"`
	Chamber   string `json:"chamber"` // "House of Representatives" or "Senate"
}

func (t Term) ContainsSession(session legislature.Session) bool {
	return t.StartYear <= session.StartYear && (t.EndYear == 0 || t.EndYear >= session.EndYear)
}

func (m Member) ToLegislatureMember() legislature.Member {
	fullName, shortName := normalizeCongressName(m.Name)

	district := normalizeShortState(m.State)
	if m.District.String() != "" {
		district += "-" + m.District.String()
	}

	return legislature.Member{
		Slug:      m.BioguideID, // i.e. S001203 - first char from last name + digits
		FullName:  fullName,
		ShortName: shortName,
		District:  district,
		Party:     normalizeParty(m.PartyName),
		URL:       m.URL,
	}
}

// normalizeCongressName takes 'last, first' -> ('last', 'first last')
func normalizeCongressName(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	last, first, _ := strings.Cut(raw, ",")
	return last, strings.TrimSpace(first + " " + last)
}

func normalizeDistrict(n json.Number, state string) string {
	state = normalizeShortState(state)
	if n.String() != "" && n.String() != "0" {
		return state + "-" + n.String()
	} else {
		return state
	}
}

func normalizeParty(p string) string {
	switch p {
	case "Democratic":
		return "D"
	case "Republican":
		return "R"
	case "Independent":
		return "I"
	default:
		return p
	}
}

// normalizeShortState converts full state names to their postal abbreviations.
// New York -> NY, ...
func normalizeShortState(s string) string {
	for _, state := range States {
		if state.Long == s {
			return state.ID
		}
	}
	return s
}
