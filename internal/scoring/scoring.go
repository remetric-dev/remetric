// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

// Package scoring computes Finding severities from analyzer observations.
// Each function takes plain primitives and returns findings.Severity.
package scoring

import "github.com/remetric-dev/remetric/internal/findings"

// CardinalitySeverity returns the tier for a metric contributing
// seriesCount out of totalSeries. Boundary semantics are strict
// greater-than (e.g. exactly 300_000 → High, not Critical).
// A non-positive totalSeries disables the percentage gates and the
// result falls back to the absolute-threshold tiers only.
func CardinalitySeverity(seriesCount, totalSeries int64) findings.Severity {
	var pct float64
	if totalSeries > 0 {
		pct = float64(seriesCount) / float64(totalSeries) * 100
	}
	switch {
	case seriesCount > 300_000 || pct > 15:
		return findings.SeverityCritical
	case seriesCount > 100_000 || pct > 5:
		return findings.SeverityHigh
	case seriesCount > 25_000:
		return findings.SeverityMedium
	default:
		return findings.SeverityLow
	}
}

// LabelPatternSeverity grades a suspicious label by its unique-value
// magnitude. The caller has already confirmed the label name matches
// a suspicious pattern; this function only ranks magnitude.
//
//	uniqueValues > 5000 → Critical
//	uniqueValues > 1000 → High
//	uniqueValues > 100  → Medium
//	otherwise           → Low
func LabelPatternSeverity(uniqueValues int) findings.Severity {
	switch {
	case uniqueValues > 5000:
		return findings.SeverityCritical
	case uniqueValues > 1000:
		return findings.SeverityHigh
	case uniqueValues > 100:
		return findings.SeverityMedium
	default:
		return findings.SeverityLow
	}
}

// UnusedMetricSeverity grades an unused metric by its head series
// count. Thresholds are lower than CardinalitySeverity because the
// false-positive risk is higher - a metric only used in an ad-hoc
// query would be flagged "unused" here, and we don't want to scream
// "Critical" at it.
//
//	seriesCount > 100_000 → Critical
//	seriesCount > 25_000  → High
//	seriesCount > 5_000   → Medium
//	otherwise             → Low
func UnusedMetricSeverity(seriesCount int64) findings.Severity {
	switch {
	case seriesCount > 100_000:
		return findings.SeverityCritical
	case seriesCount > 25_000:
		return findings.SeverityHigh
	case seriesCount > 5_000:
		return findings.SeverityMedium
	default:
		return findings.SeverityLow
	}
}
