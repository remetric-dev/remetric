// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package labelpattern

import "testing"

func TestDefaultPatterns_MatchesSuspicious(t *testing.T) {
	pats := DefaultPatterns()

	suspicious := []string{
		"user_id",
		"USER_UUID",
		"trace_id",
		"span_id",
		"request_uuid",
		"path",
		"url",
		"uri",
		"endpoint",
		"session_token",
		"request_path",
		"email",
		"username",
		"user",
	}
	for _, name := range suspicious {
		if !MatchesAny(pats, name) {
			t.Errorf("MatchesAny(%q) = false, want true", name)
		}
	}
}

func TestDefaultPatterns_IgnoresBenign(t *testing.T) {
	pats := DefaultPatterns()

	benign := []string{
		"status",
		"code",
		"method",
		"handler",
		"le",
		"quantile",
		"namespace",
		"job",
		"instance",
	}
	for _, name := range benign {
		if MatchesAny(pats, name) {
			t.Errorf("MatchesAny(%q) = true, want false", name)
		}
	}
}

func TestIsBounded_BlocksKnownLabels(t *testing.T) {
	cases := []struct {
		name string
		want bool
	}{
		{"cluster", true},
		{"environment", true},
		{"region", true},
		{"namespace", true},
		{"job", true},
		{"instance", true},
		{"user_id", false},
		{"trace_id", false},
		{"", false},
	}
	for _, c := range cases {
		got := IsBounded(c.name)
		if got != c.want {
			t.Errorf("IsBounded(%q) = %v, want %v", c.name, got, c.want)
		}
	}
}
