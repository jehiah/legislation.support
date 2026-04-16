package main

import (
	"context"
	"flag"
	"fmt"
	"sort"
	"strings"

	"github.com/jehiah/legislation.support/internal/account"
	"github.com/jehiah/legislation.support/internal/datastore"
	"github.com/jehiah/legislation.support/internal/legislature"
	"github.com/jehiah/legislation.support/internal/resolvers"
	"github.com/jehiah/legislation.support/internal/resolvers/congress"
	log "github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"
)

func main() {
	profileIDStr := flag.String("profile-id", "test-jehiah", "profile id")
	bodyStr := flag.String("body", "us-house", "legislative body")
	flag.Parse()
	log.SetFormatter(&log.TextFormatter{TimestampFormat: "2006/01/02 15:04:05", FullTimestamp: true})
	log.SetLevel(log.DebugLevel)
	ctx := context.Background()
	db := datastore.New(datastore.NewClient(ctx))

	profileID := account.ProfileID(*profileIDStr)
	if !account.IsValidProfileID(profileID) {
		log.Fatalf("invalid profile id %s", profileID)
	}
	body := legislature.BodyID(*bodyStr)
	if body != "us-house" {
		log.Fatalf("body %q not supported; only us-house is implemented", body)
	}

	profile, err := db.GetProfile(ctx, profileID)
	if err != nil {
		log.Fatal(err)
	}
	if profile == nil {
		log.Fatalf("profile %s not found", profileID)
	}

	// load active bookmarks for this body
	bookmarks, err := db.GetProfileBookmarks(ctx, profileID)
	if err != nil {
		log.WithField("profileID", profileID).Fatalf("%s", err)
	}
	bookmarks = bookmarks.Active()
	var records account.Bookmarks
	for _, bb := range bookmarks {
		if bb.BodyID != body {
			continue
		}
		records = append(records, bb)
	}
	if len(records) == 0 {
		log.Info("no matching bookmarks")
		return
	}
	// keep bookmarks sorted similarly to the HTML scorecard
	sort.Sort(account.SortedBookmarks(records))

	// build people (members) and per-bill sponsor and vote data
	api := congress.NewAPI("")
	people, bills, err := buildHouseScorecardData(ctx, api, records)
	if err != nil {
		log.WithError(err).Fatal("failed to build scorecard data")
	}

	f := excelize.NewFile()
	defer f.Close()

	sheetName := "Scorecard"
	index, err := f.NewSheet(sheetName)
	if err != nil {
		log.WithError(err).Fatal("failed to create new sheet")
	}

	if err := writeScorecardSheet(f, sheetName, people, bills); err != nil {
		log.WithError(err).Fatal("failed to write scorecard sheet")
	}

	f.SetActiveSheet(index)

	// Save spreadsheet by the given path.
	filename := fmt.Sprintf("%s-%s.xlsx", profileID, body)
	if err := f.SaveAs(filename); err != nil {
		log.WithError(err).Fatal("failed to save xls")
	}
	log.WithField("file", filename).Info("wrote scorecard xlsx")

}

type voteEvent struct {
	Label       string
	MemberVotes map[string]string // key: bioguide ID
}

type billData struct {
	Bookmark   account.Bookmark
	Sponsors   map[string]bool // key: bioguide ID
	CoSponsors map[string]bool // key: bioguide ID
	Votes      []voteEvent     // one per recorded vote
}

// buildHouseScorecardData fetches the list of House members and, for each
// bookmarked bill, the sponsors and recorded votes.
func buildHouseScorecardData(ctx context.Context, api *congress.CongressAPI, bookmarks account.Bookmarks) ([]legislature.Member, []billData, error) {
	houseAPI := congress.NewHouse(resolvers.USHouse, "")
	people, err := houseAPI.Members(ctx, congress.Sessions.Current())
	if err != nil {
		return nil, nil, err
	}

	var bills []billData
	for _, bm := range bookmarks {
		if bm.Legislation == nil {
			continue
		}

		bd := billData{
			Bookmark:   bm,
			Sponsors:   make(map[string]bool),
			CoSponsors: make(map[string]bool),
		}

		// fetch actions and recorded votes for this bill
		bill, err := api.GetBillByID(ctx, bm.Legislation.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("GetBillByID %s: %w", bm.Legislation.ID, err)
		}
		for _, s := range bill.Sponsors {
			bd.Sponsors[s.BioguideID] = true
		}
		for _, s := range bill.Cosponsors.Items {
			bd.CoSponsors[s.BioguideID] = true
		}

		actions, err := api.GetActions(ctx, bill.Congress, bill.Type, bill.Number)
		if err != nil {
			return nil, nil, fmt.Errorf("GetActions %d %s %s: %w", bill.Congress, bill.Type, bill.Number, err)
		}

		for _, action := range actions {
			// if isVoteIrrelevant(action.Text) {
			// 	continue
			// }
			for _, rv := range action.RecordedVotes {
				if !isHouseChamber(rv.Chamber) {
					continue
				}
				vote, err := api.GetVoteXML(ctx, rv.URL)
				if err != nil {
					return nil, nil, fmt.Errorf("GetVoteXML %s: %w", rv.URL, err)
				}
				ve := voteEvent{
					Label:       buildVoteLabel(vote),
					MemberVotes: make(map[string]string),
				}
				for _, m := range vote.Members {
					id := strings.TrimSpace(m.LISMemberID)
					if id == "" {
						continue
					}
					ve.MemberVotes[id] = normalizeVoteCast(m.VoteCast)
				}
				bd.Votes = append(bd.Votes, ve)
			}
		}

		bills = append(bills, bd)
	}

	return people, bills, nil
}

func writeScorecardSheet(f *excelize.File, sheet string, people []legislature.Member, bills []billData) error {
	// header row
	row := 3
	if err := setCell(f, sheet, 1, row, "Member"); err != nil {
		return err
	}
	if err := setCell(f, sheet, 2, row, "District"); err != nil {
		return err
	}
	if err := setCell(f, sheet, 3, row, "Party"); err != nil {
		return err
	}

	col := 4
	for _, b := range bills {
		if err := setCell(f, sheet, col, 1, b.Bookmark.LegislationID); err != nil {
			return err
		}
		if err := setCell(f, sheet, col, 2, b.Bookmark.Legislation.Title); err != nil {
			return err
		}

		// sponsors column
		if err := setCell(f, sheet, col, row, "sponsors"); err != nil {
			return err
		}
		col++
		// one column per recorded vote
		for _, v := range b.Votes {
			if err := setCell(f, sheet, col, row, v.Label); err != nil {
				return err
			}
			col++
		}
	}

	// data rows: one per member
	for i, p := range people {
		row = 4 + i
		log.Infof("row %d: writing member %s %#v", row, p.FullName, p)
		if err := setCell(f, sheet, 1, row, p.FullName); err != nil {
			return err
		}
		if err := setCell(f, sheet, 2, row, p.District); err != nil {
			return err
		}
		if err := setCell(f, sheet, 3, row, p.Party); err != nil {
			return err
		}

		memberID := p.ID()
		col = 4
		for _, b := range bills {
			// sponsors column
			val := ""
			if b.Sponsors[memberID] {
				val = "Sponsor"
			}
			if b.CoSponsors[memberID] {
				val = "Co-Sponsor"
			}
			if err := setCell(f, sheet, col, row, val); err != nil {
				return err
			}
			col++
			// vote columns
			for _, v := range b.Votes {
				vote := v.MemberVotes[memberID]
				if err := setCell(f, sheet, col, row, vote); err != nil {
					return err
				}
				col++
			}
		}
	}
	return nil
}

func setCell(f *excelize.File, sheet string, col, row int, value interface{}) error {
	cell, err := excelize.CoordinatesToCellName(col, row)
	if err != nil {
		return err
	}
	return f.SetCellValue(sheet, cell, value)
}

func isVoteIrrelevant(text string) bool {
	switch {
	case strings.HasPrefix(text, "Motion to reconsider laid on the table"):
		return true
	}
	return false
}

func isHouseChamber(chamber string) bool {
	if chamber == "" {
		return true
	}
	return strings.EqualFold(chamber, "House")
}

func normalizeVoteCast(vote string) string {
	switch strings.ToLower(strings.TrimSpace(vote)) {
	case "yea", "yes", "aye":
		return "Aye"
	case "nay", "no":
		return "Nay"
	case "present":
		return "Present"
	case "not voting", "absent", "not-voting":
		return "Not Voting"
	default:
		return strings.TrimSpace(vote)
	}
}

func buildVoteLabel(v *congress.RollCallVote) string {
	parts := []string{}
	if v.VoteDate != "" {
		parts = append(parts, v.VoteDate)
	}
	switch {
	case v.VoteTitle != "":
		parts = append(parts, v.VoteTitle)
	case v.VoteQuestionText != "":
		parts = append(parts, v.VoteQuestionText)
	}

	if len(parts) == 0 {
		return "Vote"
	}
	return strings.Join(parts, ": ")
}

// normalizeCongressName takes 'last, first' -> ('last', 'first last')
func normalizeCongressName(raw string) (string, string) {
	raw = strings.TrimSpace(raw)
	last, first, _ := strings.Cut(raw, ",")
	return last, strings.TrimSpace(first + " " + last)
}

func normalizeParty(p string) string {
	switch p {
	case "Democratic":
		return "D"
	case "Republican":
		return "R"
	case "Independent":
		return "I"
	default:
		return p
	}
}

func buildDistrict(state, district string) string {
	state = strings.TrimSpace(state)
	district = strings.TrimSpace(district)
	if district == "" || district == "0" {
		return state
	}
	return fmt.Sprintf("%s-%s", state, district)
}
