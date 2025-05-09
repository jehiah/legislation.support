package nyc

import (
	"context"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

func (n NYC) LookupLegistarLegislationDetail(ctx context.Context, u *url.URL) (*url.URL, error) {
	ctx, cancel := context.WithTimeout(ctx, time.Second*3)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "https://legislation.support/")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	log.Printf("got status code %d for %s", resp.StatusCode, u.String())
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("status code %d", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return nil, nil
	}
	limitReader := io.LimitReader(resp.Body, 1024768)
	body, err := io.ReadAll(limitReader)
	if err != nil {
		return nil, err
	}
	// Expecting The New York City Council - File #: Int 1234-2020
	// Expecting The New York City Council - File #: Res 1234-2020
	title := ParseTitle(body)
	log.Printf("got title %q", title)
	prefix := "The New York City Council - File #: "
	if !strings.HasPrefix(title, prefix) {
		log.Printf("title not found")
		return nil, nil
	}
	title = strings.TrimPrefix(title, prefix)

	fileNo := strings.TrimPrefix(title, "Int ")
	if strings.HasPrefix(fileNo, "Res ") {
		fileNo = strings.ReplaceAll(fileNo, "Res ", "res-")
	}
	log.Printf("fileNo %#v", fileNo)
	// convert to an intro.nyc link
	return &url.URL{
		Scheme: "https",
		Host:   "intro.nyc",
		Path:   "/" + fileNo,
	}, nil
}

var (
	re = regexp.MustCompile(`<title[^>]*?>([^<]+)<`)
)

func ParseTitle(body []byte) string {
	matches := re.FindSubmatch(body)
	if matches == nil {
		return ""
	}
	return strings.TrimSpace(html.UnescapeString(string(matches[1])))
}
