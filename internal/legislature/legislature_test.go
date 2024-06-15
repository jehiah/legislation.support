package legislature

import (
	"fmt"
	"reflect"
	"testing"
	"time"
)

func TestCalculateSponsorChanges(t *testing.T) {
	tests := []struct {
		name string
		a, b Legislation
		want []SponsorChange
	}{
		{
			name: "no changes",
		},
		{
			name: "add a sponsor",
			b: Legislation{
				Sponsors: []Member{
					{NumericID: 1},
				},
			},
			want: []SponsorChange{
				{Member: Member{NumericID: 1}},
			},
		},
		{
			name: "change details",
			a: Legislation{
				Sponsors: []Member{
					{NumericID: 1},
				},
			},
			b: Legislation{
				Sponsors: []Member{
					{NumericID: 1, FullName: "New Name"},
				},
			},
		},
		// TODO: Add test cases.
	}
	for i, tc := range tests {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			t.Log(tc.name)
			got := CalculateSponsorChanges(tc.a, tc.b)
			for i := range got {
				got[i].Date = time.Time{}
			}
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("CalculateSponsorChanges() = %#v, want %v", got, tc.want)
			}
		})
	}
}
