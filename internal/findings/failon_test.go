// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package findings_test

import (
	"testing"

	"github.com/remetric-dev/remetric/internal/findings"
)

func TestParseFailOn(t *testing.T) {
	tests := []struct {
		in          string
		wantSev     findings.Severity
		wantEnabled bool
		wantErr     bool
	}{
		{"", 0, false, false},
		{"none", 0, false, false},
		{"None", 0, false, false},
		{" none ", 0, false, false},
		{"low", findings.SeverityLow, true, false},
		{"medium", findings.SeverityMedium, true, false},
		{"HIGH", findings.SeverityHigh, true, false},
		{"critical", findings.SeverityCritical, true, false},
		{"nope", 0, false, true},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			sev, enabled, err := findings.ParseFailOn(tc.in)
			if (err != nil) != tc.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, tc.wantErr)
			}
			if err != nil {
				return
			}
			if sev != tc.wantSev {
				t.Errorf("sev = %v, want %v", sev, tc.wantSev)
			}
			if enabled != tc.wantEnabled {
				t.Errorf("enabled = %v, want %v", enabled, tc.wantEnabled)
			}
		})
	}
}

func TestShouldFail(t *testing.T) {
	mk := func(sevs ...findings.Severity) []findings.Finding {
		out := make([]findings.Finding, len(sevs))
		for i, s := range sevs {
			out[i] = findings.Finding{Severity: s}
		}
		return out
	}
	tests := []struct {
		name      string
		threshold findings.Severity
		enabled   bool
		fs        []findings.Finding
		want      bool
	}{
		{"disabled with critical findings", findings.SeverityCritical, false, mk(findings.SeverityCritical), false},
		{"enabled no findings", findings.SeverityCritical, true, nil, false},
		{"threshold met exactly", findings.SeverityHigh, true, mk(findings.SeverityHigh), true},
		{"threshold exceeded", findings.SeverityMedium, true, mk(findings.SeverityCritical), true},
		{"threshold not met", findings.SeverityCritical, true, mk(findings.SeverityHigh), false},
		{"mixed, one meets", findings.SeverityHigh, true, mk(findings.SeverityLow, findings.SeverityHigh, findings.SeverityMedium), true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := findings.ShouldFail(tc.threshold, tc.enabled, tc.fs); got != tc.want {
				t.Errorf("ShouldFail(%v, %v, %d findings) = %v, want %v",
					tc.threshold, tc.enabled, len(tc.fs), got, tc.want)
			}
		})
	}
}
