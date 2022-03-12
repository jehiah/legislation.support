package nyc

import (
	"context"
	"fmt"
	"html"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
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
	log.Printf("got status code %d", resp.StatusCode)
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("Got status code %d", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return nil, nil
	}
	limitReader := io.LimitReader(resp.Body, 1024768)
	body, err := ioutil.ReadAll(limitReader)
	if err != nil {
		return nil, err
	}
	// Expecting The New York City Council - File #: Int 1141-2018
	title := ParseTitle(body)
	log.Printf("got body %q", title)
	prefix := "The New York City Council - File #: Int "
	if strings.HasPrefix(title, prefix) {
		fileNo := strings.TrimPrefix(title, prefix)
		log.Printf("fileNo %#v", fileNo)
		// convert to an intro.nyc link
		return &url.URL{
			Scheme: "https",
			Host:   "intro.nyc",
			Path:   "/" + fileNo,
		}, nil
	}
	return nil, nil
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
