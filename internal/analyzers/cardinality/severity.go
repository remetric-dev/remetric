// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

// Package cardinality identifies high-cardinality metrics and attributes them
// to the label that drives the explosion.
package cardinality

import "github.com/remetric-dev/remetric/internal/findings"

// severity returns the severity tier for a metric contributing seriesCount
// out of totalSeries, per the spec thresholds. The boundary semantics are
// strict greater-than (e.g. exactly 300_000 → High, not Critical).
func severity(seriesCount, totalSeries int64) findings.Severity {
	if totalSeries <= 0 {
		totalSeries = 1
	}
	pct := float64(seriesCount) / float64(totalSeries) * 100
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
