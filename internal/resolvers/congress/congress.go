package congress

// https://www.congress.gov/bill/117th-congress/house-bill/8555?s=4&r=1
// propublica

// https://api.propublica.org/congress/{version}/
// GET https://api.propublica.org/congress/v1/bills/search.json?query={query}
// "https://api.propublica.org/congress/v1/117/bills/hr8555.json",

// ENV PRO_PUBLICA_CONGRESS_API_KEY

// curl -H "X-API-Key: $PRO_PUBLICA_CONGRESS_API_KEY" "https://api.propublica.org/congress/v1/117/bills/hr8555.json"
