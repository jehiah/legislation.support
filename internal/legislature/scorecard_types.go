package legislature

import (
	"strings"
)

type ScorecardMetadata struct {
	PersonTitle string
}

type ScorecardPerson struct {
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
	Metadata ScorecardMetadata
	People   []ScorecardPerson
	Data     []ScoredBookmark
}

type Score struct {
	Status  string
	Desired bool
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

func (c ScoredBookmark) PercentCorrect() float64 {
	if len(c.Scores) == 0 {
		return 0
	}
	have := 0
	for _, s := range c.Scores {
		if s.Score() == 1 {
			have++
		}
	}
	return (float64(have) / float64(len(c.Scores))) * 100
}

func (c Scorecard) PercentCorrect(idx int) float64 {
	if len(c.Data) == 0 {
		return 0
	}
	have := 0
	for _, cc := range c.Data {
		if cc.Scores[idx].Score() == 1 {
			have++
		}
	}
	return (float64(have) / float64(len(c.Data))) * 100
}
