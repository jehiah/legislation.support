package congress

import (
	"time"

	"github.com/jehiah/legislation.support/internal/legislature"
)

// Sessions defines the congressional sessions (each congress is 2 years)
var Sessions = legislature.Sessions{
	{StartYear: 2025, EndYear: 2026}, // 119th
	{StartYear: 2023, EndYear: 2024}, // 118th
	{StartYear: 2021, EndYear: 2022}, // 117th
	{StartYear: 2019, EndYear: 2020}, // 116th
	{StartYear: 2017, EndYear: 2018}, // 115th
	{StartYear: 2015, EndYear: 2016}, // 114th
	{StartYear: 2013, EndYear: 2014}, // 113th
	{StartYear: 2011, EndYear: 2012}, // 112th
}

func congressNumber(s legislature.Session) int {
	if s.StartYear < 1786 {
		s.StartYear = time.Now().Year()
	}
	return ((s.StartYear - 1789) / 2) + 1
}

func SessionForCongress(congress int) legislature.Session {
	return Sessions.Find(congress*2 + 1787)
}
