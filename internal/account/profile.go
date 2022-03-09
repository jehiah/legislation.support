package account

import "time"

type Profile struct {
	Name    string
	ID      string
	Private bool
	UID     string

	Created      time.Time
	LastModified time.Time

	// Colors?
}
