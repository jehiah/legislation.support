package legislature

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"
)

type LegislationType string

var (
	ResolutionType LegislationType = "Resolution"
	BillType       LegislationType = "Bill"
	UnknownType    LegislationType = ""
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
	Status      string
	Type        LegislationType // i.e. Resolution, Bill, etc
	// Committee ?
	// Prime Sponsor
	Sponsors []Member

	// for Bicameral legislatures
	SameAs        LegislationID // the bill in the other house (if exists)
	SubstitutedBy LegislationID // in some legislatures a bill is substituted for a bill in the other house

	// status?
	// dates?
	IntroducedDate time.Time
	LastModified   time.Time

	// legislation.support dates
	Added       time.Time
	LastChecked time.Time // when we last checked for updates
}

// func (l Legislation) IsStale(cutoff time.Time) bool {
// 	if l.LastChecked.IsZero() {
// 		return true
// 	}
// 	if len(l.Sponsors) == 0 {
// 		return true
// 	}

// 	if !l.Session.Active() {
// 		return false
// 	}

// 	// // shorter timeframe for bicameral bills that don't have SameAs yet
// 	// if l.SameAs == "" && l.Body != "nyc" {
// 	// 	// TODO: don't hard code body
// 	// 	target = time.Hour * 24 * 2
// 	// }

// 	if l.LastChecked.Before(cutoff) {
// 		return true
// 	}
// 	return false
// }

// func (l Legislation) Key() string {
// 	return string(l.Body) + "." + string(l.ID)
// }

type BodyID string        // i.e. "nyc"
type LegislationID string // i.e. 1234-456 (must not contain a '/') must be unique across sessions
type GlobalID struct {
	BodyID
	LegislationID
}

func (g GlobalID) String() string {
	return fmt.Sprintf("%s.%s", g.BodyID, g.LegislationID)
}

// Body represents a specific legislature
type Body struct {
	ID         BodyID
	DisplayID  string
	Name       string
	Location   string // ex: New York
	URL        string
	MemberName string
	Bicameral  BodyID                       // In a bicameral legislature, the other half
	UpperHouse bool                         // In a bicameral legislature, the upper house
	Sort       func(a, b *Legislation) bool `json:"-"`
}

type Resolver interface {
	Lookup(ctx context.Context, u *url.URL) (*Legislation, error)
	Refresh(context.Context, LegislationID) (*Legislation, error)
	Body() Body
	Scorecard(context.Context, []Scorable) (*Scorecard, error)
	Members(context.Context, Session) ([]Member, error)
	// Votes

	Link(l LegislationID) *url.URL
	DisplayID(l LegislationID) string
	SupportedDomains() []string
}
type Resolvers []Resolver

func (r Resolvers) Find(ID BodyID) Resolver {
	for _, rr := range r {
		if rr.Body().ID == ID {
			return rr
		}
	}
	return nil
}

func (r Resolvers) SupportedDomains() []string {
	var domains []string
	for _, rr := range r {
		domains = append(domains, rr.SupportedDomains()...)
	}
	return domains
}

func (r Resolvers) Lookup(ctx context.Context, u *url.URL) (*Legislation, error) {
	var e error
	for _, rr := range r {
		d, err := rr.Lookup(ctx, u)
		if err != nil {
			e = err
			// try others first and defer last error till end
			continue
		}
		if d != nil {
			return d, nil
		}
	}
	return nil, e
}

// MetadataSite is a site that indexes legislation but is not authoritative (i.e. indexes bills from multiple legislators)
//
// i.e. https://www.billtrack50.com
type MetadataSite interface {
	Lookup(ctx context.Context, u *url.URL) (*url.URL, error)
	SupportedDomains() []string
}
type MetadataSites []MetadataSite

// Lookup returns a URL to the canonical source of the legislation (or returns u)
func (m MetadataSites) Lookup(ctx context.Context, u *url.URL) (*url.URL, error) {
	var e error
	for _, mm := range m {
		d, err := mm.Lookup(ctx, u)
		if err != nil {
			e = err
			// try others first and defer last error till end
			continue
		}
		if d != nil {
			return d, nil
		}
	}
	return u, e
}

func (m MetadataSites) SupportedDomains() []string {
	var domains []string
	for _, mm := range m {
		domains = append(domains, mm.SupportedDomains()...)
	}
	return domains
}

func GenericLegislationSort(a, b *Legislation) bool {
	switch {
	case a.Body != b.Body:
		return a.Body < b.Body
	case a.Session != b.Session:
		return a.Session.StartYear < b.Session.StartYear
	default:
		return a.ID < b.ID
	}
}

var ErrNotFound = errors.New("Not Found")

type Session struct {
	StartYear, EndYear int // inclusive
}

func (s Session) Active() bool {
	now := time.Now().UTC().Year()
	return s.EndYear >= now && s.StartYear <= now
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
func (s Session) Overlaps(start, end time.Time) bool {
	sy, ey := start.Year(), end.Year()
	switch {
	case sy >= s.StartYear && sy <= s.EndYear:
		return true
	case ey >= s.StartYear && ey <= s.EndYear:
		return true
	case sy < s.StartYear && ey > s.EndYear:
		return true
	}
	return false
}

type Member struct {
	NumericID int    `firestore:",omitempty"`
	Slug      string `firestore:",omitempty"`
	FullName  string `firestore:",omitempty"`
	ShortName string `firestore:",omitempty"`
	URL       string `firestore:",omitempty"`
	District  string `firestore:",omitempty"`
	// TODO: party? parties?
	// Todo body?
}

func (m Member) ID() string {
	if m.NumericID != 0 {
		return fmt.Sprintf("%d", m.NumericID)
	}
	return m.Slug
}

type SponsorChange struct {
	Withdraw bool `firestore:",omitempty"`
	Date     time.Time
	Member   Member
}

// CalculateSponsorChanges returns a list of changes in .Sponsors from a to b
func CalculateSponsorChanges(a, b Legislation) []SponsorChange {
	have := make(map[string]bool, len(a.Sponsors))
	members := make(map[string]Member, len(a.Sponsors))
	var changes []SponsorChange
	for _, m := range a.Sponsors {
		have[m.ID()] = true
		members[m.ID()] = m
	}
	date := b.LastModified
	if date.IsZero() {
		date = time.Now().UTC()
	}
	for _, m := range b.Sponsors {
		if !have[m.ID()] {
			changes = append(changes, SponsorChange{Date: date, Member: m})
		}
		have[m.ID()] = false
	}
	for memberID, v := range have {
		if v {
			changes = append(changes, SponsorChange{Date: date, Member: members[memberID], Withdraw: true})
		}
	}
	return changes
}

type Changes struct {
	Sponsors []SponsorChange
}

type ResubmitMapping map[GlobalID]GlobalID

func (m ResubmitMapping) Extend(mm ResubmitMapping) {
	for k, v := range mm {
		m[k] = v
	}
}
