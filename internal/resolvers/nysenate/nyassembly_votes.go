package nysenate

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/jehiah/legislation.support/internal/legislature"
	log "github.com/sirupsen/logrus"
	"golang.org/x/net/html"
)

// AssemblyVotes returns votes for an assembly bill
//
// see https://github.com/nysenate/OpenLegislation/issues/122
//
// https://nyassembly.gov/leg/?default_fld=&leg_video=&bn=A09275&term=2021&Committee%26nbspVotes=Y&Floor%26nbspVotes=Y
func (a NYSenateAPI) AssemblyVotes(ctx context.Context, members []legislature.Member, session, printNo string) (*Bill, error) {
	u := "https://nyassembly.gov/leg/?" + url.Values{
		"default_fld":         []string{""},
		"leg_video":           []string{""},
		"bn":                  []string{printNo},
		"term":                []string{session},
		"Committee&nbspVotes": []string{"Y"},
		"Floor&nbspVotes":     []string{"Y"},
	}.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var bill Bill
	bill.Votes.Items, err = parseAssemblyVotes(resp.Body, members)
	log.WithContext(ctx).WithField("nyassembly", u).WithField("votes", len(bill.Votes.Items)).Info("looking up bill")
	return &bill, err
}

func parseAssemblyVotes(r io.Reader, members []legislature.Member) ([]BillVote, error) {
	memberLookup := make(map[string]int)
	for _, m := range members {
		memberLookup[m.ShortName] = m.NumericID
	}
	var out []BillVote
	z := html.NewTokenizer(r)
	var inTable, inCaption, date bool
	var text, dateStr, caption string
	var tokens []string
	for {
		tt := z.Next()
		token := z.Token()
		switch tt {
		case html.ErrorToken:
			err := z.Err()
			if err == io.EOF {
				err = nil
			}
			return out, err
		case html.TextToken:
			text += strings.TrimSpace(token.Data)
			switch {
			case inCaption && text == "DATE:":
				date = true
			case inCaption && date:
				dateStr = text
				date = false
			case inCaption:
				caption += token.Data
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			switch token.Data {
			case "table":
				inTable = true
			case "td":
				text = ""
			case "caption":
				inCaption = true
			}
		case html.EndTagToken:
			switch token.Data {
			case "td":
				if text != "" && inTable {
					tokens = append(tokens, text)
					// log.Printf("td %s", text)
				}
			case "table":
				inTable = false
				bv := BillVote{
					VoteDate: dateStr,
				}
				bv.Committee.Chamber = "ASSEMBLY"
				bv.Committee.Name = caption
				mv := MemberVotes{}
				for i := 0; i+1 < len(tokens); i += 2 {
					shortName := strings.ToUpper(tokens[i])
					switch tokens[i+1] {
					case "Y", "Aye":
						mv.Aye.Items = append(mv.Aye.Items, MemberEntry{
							MemberID:  memberLookup[shortName],
							Chamber:   bv.Committee.Chamber,
							ShortName: shortName,
						})
					case "N", "NO", "Nay":
						mv.Nay.Items = append(mv.Nay.Items, MemberEntry{
							MemberID:  memberLookup[shortName],
							Chamber:   bv.Committee.Chamber,
							ShortName: shortName,
						})
					case "ER", "Excused":
						mv.Excused.Items = append(mv.Excused.Items, MemberEntry{
							MemberID:  memberLookup[shortName],
							Chamber:   bv.Committee.Chamber,
							ShortName: shortName,
						})
					default:
						log.WithField("caption", caption).Infof("unkown td %q %q", tokens[i], tokens[i+1])
					}
				}
				bv.MemberVotes.Items = mv
				out = append(out, bv)
			case "caption":
				inCaption = false
			}
		}
	}
	return out, nil
}
