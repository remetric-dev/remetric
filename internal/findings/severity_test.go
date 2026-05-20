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

func TestSeverity_UnmarshalJSON_Null(t *testing.T) {
	s := SeverityHigh
	if err := json.Unmarshal([]byte(`null`), &s); err != nil {
		t.Errorf("Unmarshal(null) error: %v, want nil", err)
	}
	if s != SeverityHigh {
		t.Errorf("Unmarshal(null) modified severity: got %v, want SeverityHigh", s)
	}
}

func TestSeverity_UnmarshalJSON_EscapedString(t *testing.T) {
	// "low" decodes to "low" at the JSON layer.
	var s Severity
	if err := json.Unmarshal([]byte(`"low"`), &s); err != nil {
		t.Fatalf("Unmarshal escaped: %v", err)
	}
	if s != SeverityLow {
		t.Errorf("Unmarshal escaped = %v, want SeverityLow", s)
	}
}

func TestSeverity_UnmarshalJSON_NonString(t *testing.T) {
	cases := []string{`123`, `true`, `["low"]`, `{"x":1}`}
	for _, in := range cases {
		var s Severity
		err := json.Unmarshal([]byte(in), &s)
		if err == nil {
			t.Errorf("Unmarshal(%s) = nil error, want error", in)
		}
	}
}

func TestSeverity_RoundTrip(t *testing.T) {
	for _, sev := range []Severity{SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical} {
		b, err := json.Marshal(sev)
		if err != nil {
			t.Fatalf("Marshal(%v): %v", sev, err)
		}
		var got Severity
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatalf("Unmarshal(%s): %v", b, err)
		}
		if got != sev {
			t.Errorf("round-trip(%v) = %v", sev, got)
		}
	}
}
