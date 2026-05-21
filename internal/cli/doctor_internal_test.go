// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package cli

import "testing"

func TestVersionParts_StripsLeadingV(t *testing.T) {
	cases := []struct {
		in   string
		want [3]int
	}{
		{"v1.99.0", [3]int{1, 99, 0}},
		{"2.50.1", [3]int{2, 50, 1}},
		{"", [3]int{}},
		{"v2.30.0-rc.1", [3]int{2, 30, 0}},
	}
	for _, tc := range cases {
		got := versionParts(tc.in)
		if got != tc.want {
			t.Errorf("versionParts(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
