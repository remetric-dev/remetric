// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package cli

import "github.com/remetric-dev/remetric/internal/findings"

// emptyCopy bundles the analyzer-specific text used when a command
// produces no findings to render.
type emptyCopy struct {
	// NoResults is shown when the analyzer itself returned 0 findings.
	NoResults string
	// Subject is the plural noun used in the filter-hint sentence,
	// e.g. "cardinality offenders", "suspicious labels".
	Subject string
}

var (
	cardinalityCopy = emptyCopy{
		NoResults: "No high-cardinality metrics found above Phase 2 thresholds.",
		Subject:   "cardinality offenders",
	}
	labelPatternCopy = emptyCopy{
		NoResults: "No suspicious labels found (no label names matched the unbounded-identifier patterns).",
		Subject:   "suspicious labels",
	}
)

// tallyBySeverity counts findings per severity tier.
func tallyBySeverity(fs []findings.Finding) map[findings.Severity]int {
	out := map[findings.Severity]int{}
	for _, f := range fs {
		out[f.Severity]++
	}
	return out
}

// filterAtLeast returns findings whose severity is min or higher.
func filterAtLeast(fs []findings.Finding, min findings.Severity) []findings.Finding {
	out := make([]findings.Finding, 0, len(fs))
	for _, f := range fs {
		if f.Severity >= min {
			out = append(out, f)
		}
	}
	return out
}
