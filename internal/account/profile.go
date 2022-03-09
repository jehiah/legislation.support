package account

import (
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

type Bookmark struct {
	Body          legislature.BodyID
	LegislationID legislature.LegislationID // Legislation Key
	UID           UID                       // User ID

	Oppose bool
	Rank   []bool // sortable

	Created      time.Time
	LastModified time.Time

	// Tags []string
	// Title?
	// Statement?
}

func (b Bookmark) Key() string {
	return string(b.Body) + "." + string(b.LegislationID)
}

func IsValidProfileID(s ProfileID) bool {
	switch s {
	case "", "sign_out", "sign_in", "about",
		"session", "static":
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
