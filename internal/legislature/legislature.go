package legislature

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"
)

type Status string

const (
	Introduced Status = "introduced"

	// in-progress states
	// ???

	// terminal states
	Withdrawn = "withdrawn"
	Enacted   = "enacted"
	Vetoed    = "vetoed"
)

// Legislation is uniquely known by an ID w/in a BODY
type Legislation struct {
	Body        BodyID
	ID          LegislationID
	DisplayID   string
	Title       string
	Summary     string
	Description string
	URL         string
	Session     Session
	Status      Status

	// status?
	// dates?
	IntroducedDate time.Time

	// legislation.support dates
	Added        time.Time
	LastModified time.Time
}

// func (l Legislation) Key() string {
// 	return string(l.Body) + "." + string(l.ID)
// }

type BodyID string        // i.e. "nyc"
type LegislationID string // i.e. 1234-456 (must not contain a '/')

// Body represents a specific legislature
type Body struct {
	ID        BodyID
	DisplayID string
	Name      string
	Location  string // ex: New York
	URL       string
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

type Session struct {
	StartYear, EndYear int // inclusive
}

func (s Session) String() string { return fmt.Sprintf("%d-%d", s.StartYear, s.EndYear) }

type Sessions []Session

func (s Sessions) Current() Session { return s.Find(time.Now().UTC().Year()) }
func (s Sessions) Find(year int) Session {
	for _, ss := range s {
		if year >= ss.StartYear && year <= ss.EndYear {
			return ss
		}
	}
	return Session{}
}
