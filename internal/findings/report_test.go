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

func TestReport_PopulatedTargetOverview(t *testing.T) {
	r := &Report{
		Version:   "0.2.0",
		ScannedAt: time.Date(2026, 5, 20, 10, 23, 45, 0, time.UTC),
		Target: &Target{
			PrometheusURL:     "http://prometheus.example.com:9090",
			PrometheusVersion: "2.51.2",
		},
		Overview: &Overview{
			TotalSeries:          2_341_892,
			TotalMetrics:         4_127,
			EstimatedMemoryBytes: 13_314_752_512,
		},
		Findings: []Finding{},
		Summary:  Summary{BySeverity: map[string]int{}, PotentialSeriesReduction: 0},
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	target, ok := got["target"].(map[string]any)
	if !ok {
		t.Fatalf("expected target object, got: %v", got["target"])
	}
	if target["prometheus_url"] != "http://prometheus.example.com:9090" {
		t.Errorf("prometheus_url = %v, want http://prometheus.example.com:9090", target["prometheus_url"])
	}
	overview, ok := got["overview"].(map[string]any)
	if !ok {
		t.Fatalf("expected overview object, got: %v", got["overview"])
	}
	if overview["total_series"] != float64(2_341_892) {
		t.Errorf("total_series = %v, want 2341892", overview["total_series"])
	}
}

func TestNewReport_DoesNotAliasInput(t *testing.T) {
	fs := []Finding{
		{ID: "a", Severity: SeverityCritical, Impact: Impact{SeriesReduction: 100}},
	}
	r := NewReport("0.2.0", fs)
	// Mutate caller's slice — Report.Findings must NOT be affected.
	fs[0].ID = "mutated"
	if r.Findings[0].ID != "a" {
		t.Errorf("Report.Findings aliased caller's slice: got ID=%q, want %q", r.Findings[0].ID, "a")
	}
}
