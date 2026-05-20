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
