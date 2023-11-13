package account

import (
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/jehiah/legislation.support/internal/legislature"
)

type UID string // A globally unique User ID
type ProfileID string

type Profile struct {
	Name    string
	ID      ProfileID
	Private bool
	UID     UID

	Created      time.Time
	LastModified time.Time

	// Colors?
}

func (p Profile) HasAccess(u UID) bool {
	return p.UID == u
}

func (p Profile) Link() string {
	return "/" + url.PathEscape(string(p.ID))
}
func (p Profile) FullLink() string {
	return "https://legislation.support/" + url.PathEscape(string(p.ID))
}

type Bookmark struct {
	BodyID        legislature.BodyID
	LegislationID legislature.LegislationID // Legislation Key
	UID           UID                       // User ID

	Oppose bool
	Rank   []bool // sortable

	Created      time.Time
	LastModified time.Time

	Tags  []string
	Notes string

	Body        *legislature.Body        `firestore:"-"`
	Legislation *legislature.Legislation `firestore:"-"`
}
type Bookmarks []Bookmark

// Filter includes items that match any of the selected bodies
func (b Bookmarks) Filter(body ...legislature.BodyID) Bookmarks {
	var out Bookmarks
	for _, bb := range b {
		for _, target := range body {
			if bb.BodyID == target {
				out = append(out, bb)
				break
			}
		}
	}
	return out
}

func (b Bookmarks) Active() Bookmarks {
	var out Bookmarks
	for _, bb := range b {
		if bb.Legislation.Session.Active() {
			out = append(out, bb)
		}
	}
	return out
}

func (b Bookmarks) Bodies() []legislature.BodyID {
	l := make(map[legislature.BodyID]bool)
	for _, bb := range b {
		l[bb.BodyID] = true
	}
	var bodies []legislature.BodyID
	for body, _ := range l {
		bodies = append(bodies, body)
	}
	return bodies
}

func (b Bookmark) NewScore() legislature.ScoredBookmark {
	return legislature.ScoredBookmark{
		Legislation: b.Legislation,
		Oppose:      b.Oppose,
		// Tags: b.Tags,
	}
}

func (b Bookmark) Key() string {
	return string(b.BodyID) + "." + string(b.LegislationID)
}

func BookmarkKey(b legislature.BodyID, l legislature.LegislationID) string {
	return string(b) + "." + string(l)
}

// SortedBookmarks implements sort.Interface
type SortedBookmarks []Bookmark

func (s SortedBookmarks) Len() int      { return len(s) }
func (s SortedBookmarks) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s SortedBookmarks) Less(i, j int) bool {
	if s[i].BodyID != s[j].BodyID {
		return s[i].Body.Name < s[j].Body.Name
	}
	return s[i].Body.Sort(s[i].Legislation, s[j].Legislation)
}

func IsValidProfileID(s ProfileID) bool {
	switch s {
	case "", "sign_out", "sign_in", "about",
		"session", "static", "search":
		return false
	}
	if strings.IndexFunc(string(s), func(r rune) bool { return (r != '-' && unicode.IsPunct(r)) || unicode.IsSpace(r) }) != -1 {
		return false
	}
	if len(s) < 3 {
		return false
	}
	return true
}
