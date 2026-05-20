// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package findings

import (
	"encoding/json"
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

func TestSeverity_MarshalJSON(t *testing.T) {
	cases := []struct {
		in   Severity
		want string
	}{
		{SeverityLow, `"low"`},
		{SeverityMedium, `"medium"`},
		{SeverityHigh, `"high"`},
		{SeverityCritical, `"critical"`},
	}
	for _, c := range cases {
		got, err := json.Marshal(c.in)
		if err != nil {
			t.Fatalf("Marshal(%v) error: %v", c.in, err)
		}
		if string(got) != c.want {
			t.Errorf("Marshal(%v) = %s, want %s", c.in, got, c.want)
		}
	}
}

func TestSeverity_UnmarshalJSON(t *testing.T) {
	cases := []struct {
		in   string
		want Severity
	}{
		{`"low"`, SeverityLow},
		{`"medium"`, SeverityMedium},
		{`"HIGH"`, SeverityHigh},
		{`"Critical"`, SeverityCritical},
	}
	for _, c := range cases {
		var got Severity
		if err := json.Unmarshal([]byte(c.in), &got); err != nil {
			t.Fatalf("Unmarshal(%s) error: %v", c.in, err)
		}
		if got != c.want {
			t.Errorf("Unmarshal(%s) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestSeverity_UnmarshalJSON_Invalid(t *testing.T) {
	var s Severity
	err := json.Unmarshal([]byte(`"nope"`), &s)
	if err == nil {
		t.Fatalf("Unmarshal(\"nope\") = nil error, want error")
	}
}
