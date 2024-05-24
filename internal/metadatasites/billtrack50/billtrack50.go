package billtrack50

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/html"
)

type BillTrack50 struct{}

var billDetailPattern = regexp.MustCompile("/billdetail/([0-9]+)$")

// LookupBill matches www.billtrack50.com
//
// Example: https://www.billtrack50.com/billdetail/12345
func (b BillTrack50) Lookup(ctx context.Context, u *url.URL) (*url.URL, error) {
	switch u.Hostname() {
	case "www.billtrack50.com":
		p := billDetailPattern.FindStringSubmatch(u.Path)
		if len(p) != 2 {
			log.Infof("no match %#v %s", p, u.String())
			return nil, nil
		}
		log.Infof("found URL %s", u.String())
		return b.billOfficialDocument(ctx, p[1])
	default:
		return nil, nil
	}
}

func (b BillTrack50) billOfficialDocument(ctx context.Context, billID string) (*url.URL, error) {
	u := fmt.Sprintf("https://www.billtrack50.com/billdetail/%s", billID)
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "https://legislation.support/")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return parseOfficialDocument(resp.Body)
}

// parseOfficialDocument returns the a.href after a h4 of "Official Document"
func parseOfficialDocument(r io.Reader) (*url.URL, error) {
	z := html.NewTokenizer(r)
	var inH4, officialDocumentLinkNext bool

	for {
		tt := z.Next()
		token := z.Token()
		switch tt {
		case html.ErrorToken:
			err := z.Err()
			if err == io.EOF {
				err = nil
			}
			return nil, err
		case html.TextToken:
			if inH4 {
				text := strings.TrimSpace(token.Data)
				if text == "Official Document" {
					officialDocumentLinkNext = true
				}
			}
		case html.StartTagToken, html.SelfClosingTagToken:
			switch token.Data {
			case "h4":
				inH4 = true
			case "a":
				if officialDocumentLinkNext {
					for _, a := range token.Attr {
						if a.Key == "href" {
							return url.Parse(a.Val)
						}
					}
				}
			}
		case html.EndTagToken:
			switch token.Data {
			case "h4":
				inH4 = false
			case "html":
				return nil, nil
			}
		}
	}
	return nil, nil
}
