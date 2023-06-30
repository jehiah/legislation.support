package nysenate

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	log "github.com/sirupsen/logrus"
)

const apiDomain = "https://legislation.nysenate.gov"

type NYSenateAPI struct {
	token string
}

func (a NYSenateAPI) GetBill(ctx context.Context, session, printNo string) (*Bill, error) {
	if session == "" || printNo == "" {
		return nil, nil
	}
	path := fmt.Sprintf("/api/3/bills/%s/%s", url.PathEscape(session), url.PathEscape(printNo))
	var data BillResponse
	err := a.get(ctx, path, nil, &data)
	return &(data.Bill), err
}

func (a NYSenateAPI) get(ctx context.Context, path string, params *url.Values, v interface{}) error {
	if params == nil {
		params = &url.Values{}
	}
	params.Set("key", a.token)
	params.Set("view", "with_refs")
	u := apiDomain + path
	log.WithContext(ctx).WithField("nysenate_api", u+"?"+params.Encode()).Info("looking up bill")
	req, err := http.NewRequestWithContext(ctx, "GET", u+"?"+params.Encode(), nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return json.NewDecoder(resp.Body).Decode(&v)
}

type BillResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	ResponseType string `json:"responseType"`
	Bill         Bill   `json:"result"`
}

// GetSameAs will return A923 for S20 for the same session
func (b Bill) GetSameAs() string {
	for _, a := range b.Amendments.Items {
		for _, i := range a.SameAs.Items {
			if i.Session == b.Session {
				return i.BasePrintNo
			}
		}
	}
	return ""
}

func (b Bill) GetSponsors() []MemberEntry {
	o := []MemberEntry{
		b.Sponsor.Member,
	}
	seen := make(map[int]bool)
	for _, a := range b.Amendments.Items {
		for _, m := range a.CoSponsors.Items {
			if seen[m.MemberID] {
				continue
			}
			seen[m.MemberID] = true
			o = append(o, m)
		}
		// TODO MultiSponsors
		if a.MultiSponsors.Size > 0 {
			log.Printf("MultiSponsors %s-%s %#v", b.Session, b.BasePrintNo, a.MultiSponsors)
			for _, m := range a.MultiSponsors.Items {
				if seen[m.MemberID] {
					continue
				}
				seen[m.MemberID] = true
				o = append(o, m)
			}
		}
	}
	// TODO AdditionalSponsors
	if b.AdditionalSponsors.Size > 0 {
		log.Printf("AdditionalSponsors %s-%s %#v", b.Session, b.BasePrintNo, b.AdditionalSponsors)
		for _, m := range b.AdditionalSponsors.Items {
			if seen[m.MemberID] {
				continue
			}
			seen[m.MemberID] = true
			o = append(o, m)
		}
	}
	return o
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
		Member MemberEntry `json:"member"`
		Budget bool        `json:"budget"`
		Rules  bool        `json:"rules"`
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
		Items map[string]struct {
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
				Items []MemberEntry `json:"items"`
				Size  int           `json:"size"`
			} `json:"coSponsors"`
			MultiSponsors struct {
				Items []MemberEntry `json:"items"`
				Size  int           `json:"size"`
			} `json:"multiSponsors"`
			UniBill  bool `json:"uniBill"`
			Stricken bool `json:"stricken"`
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
						Items []MemberEntry `json:"items"`
						Size  int           `json:"size"`
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
		Items []MemberEntry `json:"items"`
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
		Items interface{} `json:"items"`
		Size  int         `json:"size"`
	} `json:"billInfoRefs"`
}

func (a NYSenateAPI) GetMembers(ctx context.Context, session, chamber string) ([]MemberShort, error) {
	if session == "" || chamber == "" {
		return nil, nil
	}
	path := fmt.Sprintf("/api/3/members/%s/%s", url.PathEscape(session), url.PathEscape(chamber))
	params := &url.Values{"full": []string{"true"}}
	var data MemberListResponse
	err := a.get(ctx, path, params, &data)
	if err != nil {
		return nil, err
	}
	var out []MemberShort
	for _, m := range data.Result.Items {
		out = append(out, m.Short())
	}
	return out, nil
}

// https://legislation.nysenate.gov/static/docs/html/members.html
type MemberSessionResponse struct {
	Success      bool          `json:"success"`
	Message      string        `json:"message"`
	ResponseType string        `json:"responseType"` // "member-sessions"
	Result       MemberSession `json:"result"`
}

type MemberListResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	ResponseType string `json:"responseType"` // "member-sessions list"
	Result       struct {
		Items []MemberSession `json:"items"`
	} `json:"result"`
}

type MemberSession struct {
	MemberID     int                      `json:"memberId"` // memberId
	Chamber      string                   `json:"chamber"`  // SENATE
	Incumbent    bool                     `json:"incumbent"`
	FullName     string                   `json:"fullName"` // "James L. Seward"
	ShortName    string                   `json:"shortName"`
	DistrictCode int                      `json:"districtCode"`
	Sessions     map[string][]MemberEntry `json:"sessionShortNameMap"`
	Person       Person                   `json:"person"`
}
type MemberEntry struct {
	MemberID        int    `json:"memberId"`
	FullName        string `json:"fullName,omitempty"`
	ShortName       string `json:"shortName"`
	SessionYear     int    `json:"sessionYear"`
	DistrictCode    int    `json:"districtCode"`
	Alternate       bool   `json:"alternate"`
	SessionMemberID int    `json:"sessionMemberId,omitempty"`
}
type Person struct {
	PersonID   int         `json:"personId"`
	FullName   string      `json:"fullName"`
	FirstName  string      `json:"firstName"`
	MiddleName string      `json:"middleName"`
	LastName   string      `json:"lastName"`
	Email      string      `json:"email"`
	Prefix     string      `json:"prefix"`
	Suffix     interface{} `json:"suffix"`
	Verified   bool        `json:"verified"`
	ImgName    string      `json:"imgName"`
}

func (m MemberSession) Short() MemberShort {
	return MemberShort{
		MemberID:  m.MemberID,
		FullName:  m.FullName,
		ShortName: m.ShortName,
		District:  m.DistrictCode,
	}
}

type MemberShort struct {
	MemberID  int
	FullName  string
	ShortName string
	District  int
}
