// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package labelpattern

import "github.com/remetric-dev/remetric/internal/findings"

// estimationMethod is the EstimationMethod identifier used by this
// analyzer. It matches the Phase 1 cardinality analyzer's identifier
// because the maths is identical: drop one label, collapse N unique
// values, retain 1/N of series.
const estimationMethod = "labeldrop_upper_bound"

// EstimateImpact returns the upper-bound reduction from dropping a
// label with uniqueValues distinct values across seriesAffected series.
// Reduction = seriesAffected * (1 - 1/uniqueValues). uniqueValues<=1
// implies no reduction.
func EstimateImpact(seriesAffected int64, uniqueValues int) findings.Impact {
	if uniqueValues <= 1 || seriesAffected <= 0 {
		return findings.Impact{EstimationMethod: estimationMethod}
	}
	reduction := seriesAffected - seriesAffected/int64(uniqueValues)
	pct := float64(reduction) / float64(seriesAffected) * 100
	return findings.Impact{
		SeriesReduction:  reduction,
		Percentage:       pct,
		EstimationMethod: estimationMethod,
	}
}
