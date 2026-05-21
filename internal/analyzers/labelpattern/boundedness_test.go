// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package labelpattern

import (
	"testing"

	"github.com/remetric-dev/remetric/internal/findings"
)

func TestLooksUnbounded(t *testing.T) {
	cases := []struct {
		name   string
		values []string
		want   bool
	}{
		{"empty input", nil, true},
		{"empty slice", []string{}, true},
		{"short digits only", []string{"1", "2", "3", "4", "5"}, false},
		{"short digits two-digit", []string{"10", "20", "30"}, false},
		{"varied length", []string{"a", "abcdefghij"}, true},
		{"alphanum with separator", []string{"user-42", "user-99", "user-123"}, true},
		{"long mean length", []string{"abcdefghijk", "mnopqrstuvw", "xyz1234567z"}, true},
		{"single short digit", []string{"7"}, false},
		{"digits and letters no separator", []string{"a1", "b2", "c3"}, false},
		{"http codes", []string{"200", "404", "500", "301"}, false},
		{"uuid-like", []string{"abc-def-123", "xyz-789-456", "qq-22-99"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := looksUnbounded(tc.values); got != tc.want {
				t.Errorf("looksUnbounded(%v) = %v, want %v", tc.values, got, tc.want)
			}
		})
	}
}

func TestDowngradeOnce(t *testing.T) {
	cases := []struct {
		in   findings.Severity
		want findings.Severity
	}{
		{findings.SeverityCritical, findings.SeverityHigh},
		{findings.SeverityHigh, findings.SeverityMedium},
		{findings.SeverityMedium, findings.SeverityLow},
		{findings.SeverityLow, findings.SeverityLow}, // clamp
	}
	for _, tc := range cases {
		if got := downgradeOnce(tc.in); got != tc.want {
			t.Errorf("downgradeOnce(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
