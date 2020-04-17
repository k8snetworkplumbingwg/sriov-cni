package utils

import (
	"testing"
)

func TestIsValidateTrunkInput(t *testing.T) {

	testCases := []struct {
		input  string
		expect bool
	}{
		{
			"10",
			true,
		},
		{
			"3,5",
			true,
		},
		{
			"3,200",
			true,
		},
		{
			"10-30",
			true,
		},
		{
			"2,4,5",
			true,
		},
		{
			"2,4,5,10-20",
			true,
		},
		{
			"0",
			false,
		},
		{
			"4095",
			false,
		},
		{
			"4096",
			false,
		},
		{
			"1,20-10",
			false,
		},
		{
			"20-4096",
			false,
		},
		{
			"garbage",
			false,
		},
		{
			"0,m,o-re,garbage",
			false,
		},
		{
			" 0, 10",
			false,
		},
	}

	for _, tc := range testCases {
		got, _ := IsValidTrunkInput(tc.input)
		if got != tc.expect {
			t.Errorf("got %v; want %v", got, tc.expect)
		}
	}
}
