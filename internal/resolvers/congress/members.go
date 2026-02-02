package congress

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strings"

	"github.com/jehiah/legislation.support/internal/legislature"
)

func (h *House) Members(ctx context.Context, session legislature.Session) ([]legislature.Member, error) {
	members, err := h.api.Members(ctx, session)
	if err != nil {
		return nil, err
	}
	var result []legislature.Member
	for _, m := range members {
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
	PartyName  string      `json:"partyName"`
	State      string      `json:"state"`
	URL        string      `json:"url"`
}

func (m Member) ToLegislatureMember() legislature.Member {
	fullName, shortName := normalizeCongressName(m.Name)
	return legislature.Member{
		Slug:      m.BioguideID, // i.e. S001203 - first char from last name + digits
		FullName:  fullName,
		ShortName: shortName,
		District:  normalizeDistrict(m.District),
		Party:     m.PartyName,
		URL:       m.URL,
	}
}

// normalizeCongressName takes 'last, first' -> ('last', 'first last')
func normalizeCongressName(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	last, first, _ := strings.Cut(raw, ",")
	return last, strings.TrimSpace(first + " " + last)
}

func normalizeDistrict(n json.Number) string {
	if n == "" {
		return ""
	}
	if n.String() == "0" {
		return ""
	}
	return n.String()
}
