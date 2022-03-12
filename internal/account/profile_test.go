package account

import (
	"fmt"
	"testing"
)

func TestIsValidProfileID(t *testing.T) {
	type testCase struct {
		have ProfileID
		want bool
	}
	tests := []testCase{
		{"a", false},
		{"a-b-c", true},
		{"!@#$", false},
		{"    ", false},
		{"sign_in", false},
	}
	for i, tc := range tests {
		tc := tc
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			t.Parallel()
			if got := IsValidProfileID(tc.have); got != tc.want {
				t.Errorf("got %v want %v for %q", got, tc.want, tc.have)
			}
		})
	}
}
