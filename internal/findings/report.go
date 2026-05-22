// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package findings

import "time"

// Report is the top-level JSON output of a scan or focused subcommand.
// Phase 2 populates Version, ScannedAt, Findings, and Summary.
// Target and Overview are pointer-optional: nil means "not populated"
// and the key is omitted from JSON. Phase 3 callers will set them via
// &Target{...} / &Overview{...} once real scan metadata is available.
type Report struct {
	Version   string    `json:"version"`
	ScannedAt time.Time `json:"scanned_at"`
	Target    *Target   `json:"target,omitempty"`
	Overview  *Overview `json:"overview,omitempty"`
	Findings  []Finding `json:"findings"`
	Summary   Summary   `json:"summary"`
	// Warnings carries non-fatal degradation messages collected from analyzers
	// (e.g., "vmalert URL not configured - recording rules ignored"). Omitted
	// from JSON when empty.
	Warnings []string `json:"warnings,omitempty"`
	// IgnoredCount is the number of findings dropped by --ignore-* patterns.
	// Set at the CLI layer after Filter.Apply; omitted from JSON when zero.
	IgnoredCount int `json:"ignored_count,omitempty"`
}

// Target identifies the scanned stack.
type Target struct {
	PrometheusURL     string `json:"prometheus_url,omitempty"`
	PrometheusVersion string `json:"prometheus_version,omitempty"`
}

// Overview is the stack-level snapshot at scan time.
type Overview struct {
	TotalSeries          int64 `json:"total_series,omitempty"`
	TotalMetrics         int64 `json:"total_metrics,omitempty"`
	EstimatedMemoryBytes int64 `json:"estimated_memory_bytes,omitempty"`
}

// Summary is the aggregated severity/impact view over Findings.
type Summary struct {
	BySeverity               map[string]int `json:"by_severity"`
	PotentialSeriesReduction int64          `json:"potential_series_reduction"`
}

// NewReport builds a Report with computed Summary. ScannedAt is set
// to time.Now().UTC(). Target and Overview are left nil and will be
// populated by the caller (Phase 3+). Findings is copied; the caller
// may safely mutate the input slice afterwards.
func NewReport(version string, fs []Finding) *Report {
	bySev := map[string]int{}
	var totalRed int64
	for _, f := range fs {
		key := f.Severity.lower()
		bySev[key]++
		totalRed += f.Impact.SeriesReduction
	}
	copied := append([]Finding(nil), fs...)
	return &Report{
		Version:   version,
		ScannedAt: time.Now().UTC(),
		Findings:  copied,
		Summary: Summary{
			BySeverity:               bySev,
			PotentialSeriesReduction: totalRed,
		},
	}
}
