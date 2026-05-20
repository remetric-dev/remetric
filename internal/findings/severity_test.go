// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package findings

import (
	"sort"
	"testing"
)

func TestSeverity_String(t *testing.T) {
	tests := []struct {
		s    Severity
		want string
	}{
		{SeverityLow, "LOW"},
		{SeverityMedium, "MEDIUM"},
		{SeverityHigh, "HIGH"},
		{SeverityCritical, "CRITICAL"},
		{Severity(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		if got := tt.s.String(); got != tt.want {
			t.Errorf("Severity(%d).String() = %q, want %q", tt.s, got, tt.want)
		}
	}
}

func TestSeverity_Order(t *testing.T) {
	in := []Severity{SeverityLow, SeverityCritical, SeverityMedium, SeverityHigh}
	want := []Severity{SeverityCritical, SeverityHigh, SeverityMedium, SeverityLow}
	sort.Slice(in, func(i, j int) bool { return in[i] > in[j] })
	for i := range in {
		if in[i] != want[i] {
			t.Errorf("sorted[%d] = %v, want %v", i, in[i], want[i])
		}
	}
}

func TestParseSeverity(t *testing.T) {
	tests := []struct {
		in      string
		want    Severity
		wantErr bool
	}{
		{"low", SeverityLow, false},
		{"medium", SeverityMedium, false},
		{"HIGH", SeverityHigh, false},
		{"critical", SeverityCritical, false},
		{" High ", SeverityHigh, false},
		{"", 0, true},
		{"bogus", 0, true},
	}
	for _, tt := range tests {
		got, err := ParseSeverity(tt.in)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseSeverity(%q) err = %v, wantErr=%v", tt.in, err, tt.wantErr)
		}
		if !tt.wantErr && got != tt.want {
			t.Errorf("ParseSeverity(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
