package legislature

import (
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
}

type BodyID int

// Body represents a specific legislature
type Body struct {
	ID       BodyID
	Name     string
	Location string // ex: New York
	URL      string
	Resolver
}

type Resolver interface {
	Lookup(u *url.URL) (*Legislation, error)
}

var ErrNotFound = errors.New("Not Found")
