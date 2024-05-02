package account

import (
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/jehiah/legislation.support/internal/legislature"
)

type UID string // A globally unique User ID
type ProfileID string

type Profile struct {
	Name        string
	Description string
	ID          ProfileID
	Private     bool
	UID         UID

	Created      time.Time
	LastModified time.Time

	// Colors?
	ScorecardOptions
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

	Body          *legislature.Body        `firestore:"-"`
	BicameralBody *legislature.Body        `firestore:"-"`
	Legislation   *legislature.Legislation `firestore:"-"`
}
type Bookmarks []Bookmark

func (b Bookmarks) DisplayTags() []DisplayTag {
	tags := make(map[DisplayTag]bool)
	for _, bb := range b {
		for _, tag := range bb.DisplayTags() {
			switch tag.Class {
			case "sponsor", "session":
				continue
			}
			tags[tag] = true
		}
	}
	out := make([]DisplayTag, 0, len(tags))
	for tag := range tags {
		out = append(out, tag)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Class != out[j].Class {
			return out[i].Class < out[j].Class
		}
		return out[i].Tag < out[j].Tag
	})
	return out
}

type DisplayTag struct {
	Tag   string
	Class string
}

func (b Bookmark) DisplayTags() []DisplayTag {
	out := make([]DisplayTag, 0, len(b.Tags))
	if b.Oppose {
		out = append(out, DisplayTag{Tag: "Oppose", Class: "oppose"})
	}
	if b.BicameralBody != nil && b.BicameralBody.UpperHouse {
		out = append(out, DisplayTag{Tag: b.BicameralBody.DisplayID, Class: "body"})
	}
	if b.Body != nil {
		out = append(out, DisplayTag{Tag: b.Body.DisplayID, Class: "body"})
	}
	if b.BicameralBody != nil && !b.BicameralBody.UpperHouse {
		out = append(out, DisplayTag{Tag: b.BicameralBody.DisplayID, Class: "body"})
	}
	if b.Legislation != nil {
		out = append(out, DisplayTag{Tag: "session:" + b.Legislation.Session.String(), Class: "session"})
		// if len(b.Legislation.Sponsors) > 0 {
		// 	out = append(out, DisplayTag{Tag: "sponsor:" + b.Legislation.Sponsors[0].FullName, Class: "sponsor"})
		// }
	}
	for _, tag := range b.Tags {
		out = append(out, DisplayTag{Tag: tag, Class: "tag"})
	}
	return out
}

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
func (b Bookmarks) CountSupported() int {
	var n int
	for _, bb := range b {
		if !bb.Oppose {
			n++
		}
	}
	return n
}
func (b Bookmarks) CountOpposed() int {
	var n int
	for _, bb := range b {
		if bb.Oppose {
			n++
		}
	}
	return n
}

func (b Bookmarks) Bodies() []legislature.BodyID {
	l := make(map[legislature.BodyID]bool)
	for _, bb := range b {
		l[bb.BodyID] = true
		if bb.Legislation.SameAs != "" && bb.Body != nil {
			l[bb.Body.Bicameral] = true
		}
	}
	bodies := make([]legislature.BodyID, 0, len(l))
	for body := range l {
		bodies = append(bodies, body)
	}
	// TODO: sort by name
	sort.Slice(bodies, func(i, j int) bool { return bodies[i] < bodies[j] })
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
func (b Bookmark) UpperBody() *legislature.Body {
	if b.BicameralBody != nil && b.BicameralBody.UpperHouse {
		return b.BicameralBody
	}
	if b.Body.Bicameral != "" && !b.Body.UpperHouse {
		return nil
	}
	return b.Body
}

func (b Bookmark) LowerBody() *legislature.Body {
	if b.BicameralBody != nil && !b.BicameralBody.UpperHouse {
		return b.BicameralBody
	}
	if b.Body.Bicameral != "" && b.Body.UpperHouse {
		return nil
	}
	return b.Body
}

func BookmarkKey(b legislature.BodyID, l legislature.LegislationID) string {
	return string(b) + "." + string(l)
}

// SortedBookmarks implements sort.Interface
type SortedBookmarks []Bookmark

func (s SortedBookmarks) Len() int      { return len(s) }
func (s SortedBookmarks) Swap(i, j int) { s[i], s[j] = s[j], s[i] }
func (s SortedBookmarks) Less(i, j int) bool {
	a, b := s[i], s[j]
	b1, b2 := a.UpperBody(), b.UpperBody()
	if b1 == nil {
		b1 = a.LowerBody()
	}
	if b2 == nil {
		b2 = b.LowerBody()
	}
	if b1.ID != b2.ID {
		if b1.Bicameral == b2.ID {
			return b1.UpperHouse
		}
		return b1.Name < b2.Name
	}

	return b1.Sort(a.Legislation, b.Legislation)
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
