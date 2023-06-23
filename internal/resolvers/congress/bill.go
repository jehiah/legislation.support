package congress

// https://projects.propublica.org/api-docs/congress-api/bills/
type Bill struct {
	BillID                 string        `json:"bill_id"`
	BillSlug               string        `json:"bill_slug"`
	Congress               string        `json:"congress"`
	Bill                   string        `json:"bill"`
	BillType               string        `json:"bill_type"`
	Number                 string        `json:"number"`
	BillURI                string        `json:"bill_uri"`
	Title                  string        `json:"title"`
	ShortTitle             string        `json:"short_title"`
	SponsorTitle           string        `json:"sponsor_title"`
	Sponsor                string        `json:"sponsor"`
	SponsorID              string        `json:"sponsor_id"`
	SponsorURI             string        `json:"sponsor_uri"`
	SponsorParty           string        `json:"sponsor_party"`
	SponsorState           string        `json:"sponsor_state"`
	GpoPdfURI              interface{}   `json:"gpo_pdf_uri"`
	CongressdotgovURL      string        `json:"congressdotgov_url"`
	GovtrackURL            string        `json:"govtrack_url"`
	IntroducedDate         string        `json:"introduced_date"`
	Active                 bool          `json:"active"`
	LastVote               interface{}   `json:"last_vote"`
	HousePassage           interface{}   `json:"house_passage"`
	SenatePassage          interface{}   `json:"senate_passage"`
	Enacted                interface{}   `json:"enacted"`
	Vetoed                 interface{}   `json:"vetoed"`
	Cosponsors             int           `json:"cosponsors"`
	CosponsorsByParty      interface{}   `json:"cosponsors_by_party"`
	WithdrawnCosponsors    int           `json:"withdrawn_cosponsors"`
	PrimarySubject         string        `json:"primary_subject"`
	Committees             string        `json:"committees"`
	CommitteeCodes         []string      `json:"committee_codes"`
	SubcommitteeCodes      []interface{} `json:"subcommittee_codes"`
	LatestMajorActionDate  string        `json:"latest_major_action_date"`
	LatestMajorAction      string        `json:"latest_major_action"`
	HousePassageVote       interface{}   `json:"house_passage_vote"`
	SenatePassageVote      interface{}   `json:"senate_passage_vote"`
	Summary                string        `json:"summary"`
	SummaryShort           string        `json:"summary_short"`
	CboEstimateURL         interface{}   `json:"cbo_estimate_url"`
	Versions               []interface{} `json:"versions"`
	Actions                []Action      `json:"actions"`
	PresidentialStatements []interface{} `json:"presidential_statements"`
	Votes                  []interface{} `json:"votes"`
}

type Action struct {
	ID          int    `json:"id"`
	Chamber     string `json:"chamber"`
	ActionType  string `json:"action_type"`
	Datetime    string `json:"datetime"`
	Description string `json:"description"`
}
