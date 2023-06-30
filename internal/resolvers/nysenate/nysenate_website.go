package nysenate

// NYSenatePerson is information managed by www.nysenate.gov including party information and social media handles
// from https://www.nysenate.gov/senators.json
// src: https://github.com/nysenate/OpenLegislation/issues/104
//
// curl -s 'https://www.nysenate.gov/senators.json' | jq -c '.[] | {full_name,open_leg_id,party}'
// {"full_name":"Joseph P. Addabbo Jr.","open_leg_id":"384","party":["D"]}
// {"full_name":"Toby Ann Stavisky","open_leg_id":"400","party":["D"]}
// {"full_name":"Kevin S. Parker","open_leg_id":"416","party":["D","WF"]}
// {"full_name":"Liz Krueger","open_leg_id":"401","party":["D","WF"]}
type NYSenatePerson struct {
	MemberID              string   `json:"open_leg_id"`
	SenateDistrict        int      `json:"senate_district"`
	SenateDistrictOrdinal string   `json:"senate_district_ordinal"`
	IsActive              bool     `json:"is_active"`
	FullName              string   `json:"full_name"`
	FirstName             string   `json:"first_name"`
	LastName              string   `json:"last_name"`
	ShortName             string   `json:"short_name"`
	Email                 string   `json:"email"`
	Parties               []string `json:"party"` // D, WPF
	Role                  string   `json:"role"`
	Summary               string   `json:"summary"`
	SenateDistrictURL     string   `json:"senate_district_url"`
	URL                   string   `json:"url"`
	Img                   string   `json:"img"`
	HeroImg               string   `json:"hero_img"`
	Palette               struct {
		Name string `json:"name"`
		Lgt  string `json:"lgt"`
		Med  string `json:"med"`
		Drk  string `json:"drk"`
	} `json:"palette"`
	Offices []struct {
		Name         string      `json:"name"`
		Street       string      `json:"street"`
		Additional   string      `json:"additional"`
		City         string      `json:"city"`
		Province     string      `json:"province"`
		PostalCode   string      `json:"postal_code"`
		Country      interface{} `json:"country"`
		ProvinceName string      `json:"province_name"`
		CountryName  string      `json:"country_name"`
		Fax          string      `json:"fax"`
		Phone        string      `json:"phone"`
	} `json:"offices"`
	SocialMedia []struct {
		Name string `json:"name"`
		URL  string `json:"url"`
	} `json:"social_media"`
}
