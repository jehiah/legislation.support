package nysenate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"

	"github.com/jehiah/legislation.support/internal/legislature"
)

const apiDomain = "https://legislation.nysenate.gov"

type NYSenateAPI struct {
	token string
}

func NewNYSenateAPI(token string) *NYSenateAPI {
	if token == "" {
		panic("missing token")
	}
	return &NYSenateAPI{
		token: token,
	}
}

var nysenatePattern = regexp.MustCompile("/legislation/bills/((199|200|201|202)[0-9])/((A|S)[0-9]+)$")

func (a NYSenateAPI) Lookup(ctx context.Context, u *url.URL) (*legislature.Legislation, error) {
	switch u.Hostname() {
	// case "legistar.council.nyc.gov":
	// 	if u.Path != "/LegislationDetail.aspx" {
	// 		return nil, legislature.ErrNotFound
	// 	}
	// todo
	// https://www.nysenate.gov/legislation/bills/2021/S7635
	case "www.nysenate.gov":
	default:
		return nil, nil
	}
	p := nysenatePattern.FindStringSubmatch(u.Path)
	if len(p) != 5 {
		return nil, nil
	}
	session, printNo := p[1], p[3]
	bill, err := a.GetBill(ctx, session, printNo)
	if err != nil {
		return nil, err
	}
	// TODO
	return nil, nil
}

func (a NYSenateAPI) GetBill(ctx context.Context, session, printNo string) (*Bill, error) {
	params := &url.Values{"key": []string{a.token}, "view": []string{"with_refs"}}
	u := apiDomain + fmt.Sprintf("/api/3/bills/%s/%s?", url.PathEscape(session), url.PathEscape(printNo)) + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var data BillResponse
	err = json.NewDecoder(resp.Body).Decode(&data)
	return &(data.Bill), err
}

type BillResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	ResponseType string `json:"responseType"`
	Bill         Bill   `json:"result"`
}

// from https://legislation.nysenate.gov/static/docs/html/bills.html
type Bill struct {
	BasePrintNo string `json:"basePrintNo"`
	Session     int    `json:"session"`
	PrintNo     string `json:"printNo"`
	BillType    struct {
		Chamber    string `json:"chamber"`
		Desc       string `json:"desc"`
		Resolution bool   `json:"resolution"`
	} `json:"billType"`
	Title             string `json:"title"`
	ActiveVersion     string `json:"activeVersion"`
	Year              int    `json:"year"`
	PublishedDateTime string `json:"publishedDateTime"`
	SubstitutedBy     struct {
		BasePrintNo string `json:"basePrintNo"`
		Session     int    `json:"session"`
	} `json:"substitutedBy"`
	Sponsor struct {
		Member struct {
			MemberID     int    `json:"memberId"`
			ShortName    string `json:"shortName"`
			SessionYear  int    `json:"sessionYear"`
			FullName     string `json:"fullName"`
			DistrictCode int    `json:"districtCode"`
		} `json:"member"`
		Budget bool `json:"budget"`
		Rules  bool `json:"rules"`
	} `json:"sponsor"`
	Summary string `json:"summary"`
	Signed  bool   `json:"signed"`
	Status  struct {
		StatusType    string      `json:"statusType"`
		StatusDesc    string      `json:"statusDesc"`
		ActionDate    string      `json:"actionDate"`
		CommitteeName string      `json:"committeeName"`
		BillCalNo     interface{} `json:"billCalNo"`
	} `json:"status"`
	Milestones struct {
		Items []struct {
			StatusType    string      `json:"statusType"`
			StatusDesc    string      `json:"statusDesc"`
			ActionDate    string      `json:"actionDate"`
			CommitteeName string      `json:"committeeName"`
			BillCalNo     interface{} `json:"billCalNo"`
		} `json:"items"`
		Size int `json:"size"`
	} `json:"milestones"`
	ProgramInfo struct {
		Name       string `json:"name"`
		SequenceNo int    `json:"sequenceNo"`
	} `json:"programInfo"`
	Amendments struct {
		Items struct {
			A struct {
				BasePrintNo    string `json:"basePrintNo"`
				Session        int    `json:"session"`
				BasePrintNoStr string `json:"basePrintNoStr"`
				PrintNo        string `json:"printNo"`
				Version        string `json:"version"`
				PublishDate    string `json:"publishDate"`
				SameAs         struct {
					Items []struct {
						BasePrintNo string `json:"basePrintNo"`
						Session     int    `json:"session"`
						PrintNo     string `json:"printNo"`
						Version     string `json:"version"`
					} `json:"items"`
					Size int `json:"size"`
				} `json:"sameAs"`
				Memo             string      `json:"memo"`
				LawSection       string      `json:"lawSection"`
				LawCode          string      `json:"lawCode"`
				ActClause        string      `json:"actClause"`
				FullTextFormats  []string    `json:"fullTextFormats"`
				FullText         string      `json:"fullText"`
				FullTextHTML     interface{} `json:"fullTextHtml"`
				FullTextTemplate interface{} `json:"fullTextTemplate"`
				CoSponsors       struct {
					Items []struct {
						MemberID     int    `json:"memberId"`
						ShortName    string `json:"shortName"`
						SessionYear  int    `json:"sessionYear"`
						FullName     string `json:"fullName"`
						DistrictCode int    `json:"districtCode"`
					} `json:"items"`
					Size int `json:"size"`
				} `json:"coSponsors"`
				MultiSponsors struct {
					Items []interface{} `json:"items"`
					Size  int           `json:"size"`
				} `json:"multiSponsors"`
				UniBill  bool `json:"uniBill"`
				Stricken bool `json:"stricken"`
			} `json:"a"`
		} `json:"items"`
		Size int `json:"size"`
	} `json:"amendments"`
	Votes struct {
		Items []struct {
			Version   string `json:"version"`
			VoteType  string `json:"voteType"`
			VoteDate  string `json:"voteDate"`
			Committee struct {
				Chamber string `json:"chamber"`
				Name    string `json:"name"`
			} `json:"committee"`
			MemberVotes struct {
				Items struct {
					EXC struct {
						Items []struct {
							MemberID    int    `json:"memberId"`
							ShortName   string `json:"shortName"`
							SessionYear int    `json:"sessionYear"`
						} `json:"items"`
						Size int `json:"size"`
					} `json:"EXC"`
					AYEWR struct {
					} `json:"AYEWR"`
					NAY struct {
					} `json:"NAY"`
					AYE struct {
					} `json:"AYE"`
				} `json:"items"`
				Size int `json:"size"`
			} `json:"memberVotes"`
		} `json:"items"`
		Size int `json:"size"`
	} `json:"votes"`
	VetoMessages struct {
		Items []struct {
			BillID struct {
				BasePrintNo string `json:"basePrintNo"`
				Session     int    `json:"session"`
				PrintNo     string `json:"printNo"`
				Version     string `json:"version"`
			} `json:"billId"`
			Year       int         `json:"year"`
			VetoNumber int         `json:"vetoNumber"`
			MemoText   string      `json:"memoText"`
			VetoType   string      `json:"vetoType"`
			Chapter    int         `json:"chapter"`
			BillPage   int         `json:"billPage"`
			LineStart  int         `json:"lineStart"`
			LineEnd    int         `json:"lineEnd"`
			Signer     string      `json:"signer"`
			SignedDate interface{} `json:"signedDate"`
		} `json:"items"`
		Size int `json:"size"`
	} `json:"vetoMessages"`
	ApprovalMessage struct {
		BillID struct {
			BasePrintNo string `json:"basePrintNo"`
			Session     int    `json:"session"`
			PrintNo     string `json:"printNo"`
			Version     string `json:"version"`
		} `json:"billId"`
		Year           int    `json:"year"`
		ApprovalNumber int    `json:"approvalNumber"`
		Chapter        int    `json:"chapter"`
		Signer         string `json:"signer"`
		Text           string `json:"text"`
	} `json:"approvalMessage"`
	AdditionalSponsors struct {
		Items []interface{} `json:"items"`
		Size  int           `json:"size"`
	} `json:"additionalSponsors"`
	PastCommittees struct {
		Items []struct {
			Chamber       string `json:"chamber"`
			Name          string `json:"name"`
			SessionYear   int    `json:"sessionYear"`
			ReferenceDate string `json:"referenceDate"`
		} `json:"items"`
		Size int `json:"size"`
	} `json:"pastCommittees"`
	Actions struct {
		Items []struct {
			BillID struct {
				BasePrintNo string `json:"basePrintNo"`
				Session     int    `json:"session"`
				PrintNo     string `json:"printNo"`
				Version     string `json:"version"`
			} `json:"billId"`
			Date       string `json:"date"`
			Chamber    string `json:"chamber"`
			SequenceNo int    `json:"sequenceNo"`
			Text       string `json:"text"`
		} `json:"items"`
		Size int `json:"size"`
	} `json:"actions"`
	PreviousVersions struct {
		Items []struct {
			BasePrintNo string `json:"basePrintNo"`
			Session     int    `json:"session"`
			PrintNo     string `json:"printNo"`
			Version     string `json:"version"`
		} `json:"items"`
		Size int `json:"size"`
	} `json:"previousVersions"`
	CommitteeAgendas struct {
		Items []struct {
			AgendaID struct {
				Number int `json:"number"`
				Year   int `json:"year"`
			} `json:"agendaId"`
			CommitteeID struct {
				Chamber string `json:"chamber"`
				Name    string `json:"name"`
			} `json:"committeeId"`
		} `json:"items"`
		Size int `json:"size"`
	} `json:"committeeAgendas"`
	Calendars struct {
		Items []struct {
			Year           int `json:"year"`
			CalendarNumber int `json:"calendarNumber"`
		} `json:"items"`
		Size int `json:"size"`
	} `json:"calendars"`
	BillInfoRefs struct {
		Items struct {
			A20982013 struct {
			} `json:"A2098-2013"`
		} `json:"items"`
		Size int `json:"size"`
	} `json:"billInfoRefs"`
}
