package legislature

import (
	"context"
	"errors"
	"net/url"
)

type Legislation struct {
	Body        BodyID
	ID          string
	Title       string
	Summary     string
	Description string
	URL         string
	Session     string
	// dates?
}

func (l Legislation) Key() string {
	return string(l.Body) + "." + l.ID
}

type BodyID string // i.e. "nyc"

// Body represents a specific legislature
type Body struct {
	ID       BodyID
	Name     string
	Location string // ex: New York
	URL      string
}

type Resolver interface {
	Lookup(ctx context.Context, u *url.URL) (*Legislation, error)
}
type Resolvers []Resolver

func (r Resolvers) Lookup(ctx context.Context, u *url.URL) (*Legislation, error) {
	for _, rr := range r {
		d, err := rr.Lookup(ctx, u)
		if err != nil {
			// try others first and defer till end?
			return nil, err
		}
		if d != nil {
			return d, nil
		}
	}
	return nil, ErrNotFound
}

var ErrNotFound = errors.New("Not Found")
