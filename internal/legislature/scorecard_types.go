package legislature

import (
	"strings"
)

type ScorecardMetadata struct {
	PersonTitle string
}

type ScorecardPerson struct {
	ID       int
	FullName string
	Party    string
	URL      string
	District string
}

type Scorable interface {
	NewScore() ScoredBookmark
}
type ScoredBookmark struct {
	Legislation *Legislation

	Status    string
	Committee string

	Oppose bool
	// Tags   []string

	Scores []Score
}

type Scorecard struct {
	Body     *Body
	Metadata ScorecardMetadata
	People   []ScorecardPerson
	Data     []ScoredBookmark
}

type Score struct {
	Status  string
	Desired bool
}

type PersonWhipCount struct {
	ScorecardPerson
	WhipCount
}

func (s Score) Score() int {
	if s.Desired {
		switch strings.ToLower(s.Status) {
		case "affirmative", "aye", "sponsor":
			return 1
		case "negative", "nay":
			return -1
		}
		return 0
	}
	switch strings.ToLower(s.Status) {
	case "affirmative", "aye", "sponsor":
		return -1
	case "negative", "nay":
		return 1
	default:
		return 0
	}
}

func (s Score) CSS() string {
	if s.Desired {
		switch strings.ToLower(s.Status) {
		case "affirmative", "aye", "sponsor":
			return "affirmative"
		case "negative", "nay":
			return "negative"
		case "":
			return ""
		default:
			return "excused"
		}
	}
	switch strings.ToLower(s.Status) {
	case "affirmative", "aye", "sponsor":
		return "negative"
	case "negative", "nay":
		return "affirmative"
	case "":
		return ""
	default:
		return "excused"
	}

}

type WhipCount struct {
	Correct   int
	Incorrect int
	Total     int
}

// PercentCorrect returns in the range [0, 100]
func (w WhipCount) PercentCorrect() float64 {
	if w.Total == 0 {
		return 0
	}
	return (float64(w.Correct) / float64(w.Total)) * 100
}

// Percent returns in the range [-100, 100]
func (w WhipCount) Percent() float64 {
	if w.Total == 0 {
		return 0
	}
	return (float64(w.Correct-w.Incorrect) / float64(w.Total)) * 100
}

func (c ScoredBookmark) WhipCount() (w WhipCount) {
	for _, s := range c.Scores {
		w.Total += 1
		switch s.Score() {
		case 1:
			w.Correct += 1
		case -1:
			w.Incorrect += 1
		}
	}
	return
}

func (c Scorecard) WhipCount(idx int) (w WhipCount) {
	for _, cc := range c.Data {
		w.Total += 1
		switch cc.Scores[idx].Score() {
		case 1:
			w.Correct += 1
		case -1:
			w.Incorrect += 1
		}
	}
	return
}
