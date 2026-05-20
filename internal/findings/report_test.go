// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package findings

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

func TestNewReport_ComputesSummary(t *testing.T) {
	fs := []Finding{
		{ID: "a", Severity: SeverityCritical, Impact: Impact{SeriesReduction: 1000}},
		{ID: "b", Severity: SeverityHigh, Impact: Impact{SeriesReduction: 500}},
		{ID: "c", Severity: SeverityHigh, Impact: Impact{SeriesReduction: 200}},
		{ID: "d", Severity: SeverityMedium, Impact: Impact{SeriesReduction: 50}},
	}

	r := NewReport("0.2.0", fs)

	if r.Version != "0.2.0" {
		t.Errorf("Version = %q, want %q", r.Version, "0.2.0")
	}
	if r.ScannedAt.IsZero() {
		t.Errorf("ScannedAt is zero")
	}
	if r.ScannedAt.Location() != time.UTC {
		t.Errorf("ScannedAt not UTC: %v", r.ScannedAt.Location())
	}
	if len(r.Findings) != len(fs) {
		t.Errorf("Findings length = %d, want %d", len(r.Findings), len(fs))
	}
	wantBySev := map[string]int{
		"critical": 1,
		"high":     2,
		"medium":   1,
	}
	if diff := cmp.Diff(wantBySev, r.Summary.BySeverity); diff != "" {
		t.Errorf("BySeverity mismatch (-want +got):\n%s", diff)
	}
	if r.Summary.PotentialSeriesReduction != 1750 {
		t.Errorf("PotentialSeriesReduction = %d, want 1750", r.Summary.PotentialSeriesReduction)
	}
}

func TestReport_JSONShape(t *testing.T) {
	r := &Report{
		Version:   "0.2.0",
		ScannedAt: time.Date(2026, 5, 20, 10, 23, 45, 0, time.UTC),
		Findings:  []Finding{},
		Summary:   Summary{BySeverity: map[string]int{}, PotentialSeriesReduction: 0},
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	// Required keys.
	for _, k := range []string{"version", "scanned_at", "findings", "summary"} {
		if _, ok := got[k]; !ok {
			t.Errorf("missing key %q in JSON: %s", k, b)
		}
	}
	// Optional zero-value keys should be omitted.
	for _, k := range []string{"target", "overview"} {
		if _, ok := got[k]; ok {
			t.Errorf("expected key %q to be omitted, got: %s", k, b)
		}
	}
}
