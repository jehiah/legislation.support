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
