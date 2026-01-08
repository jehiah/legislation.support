package congress

import (
	"time"

	"github.com/jehiah/legislation.support/internal/legislature"
)

func congressNumber(s legislature.Session) int {
	if s.StartYear < 1786 {
		s.StartYear = time.Now().Year()
	}
	return ((s.StartYear - 1789) / 2) + 1
}
