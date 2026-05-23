// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/remetric-dev/remetric/internal/config"
	"github.com/remetric-dev/remetric/internal/findings"
	outjson "github.com/remetric-dev/remetric/internal/output/json"
)

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
	unusedMetricsCopy = emptyCopy{
		NoResults: "No unused metrics found - every ingested metric is referenced by a dashboard, alert, or recording rule.",
		Subject:   "unused metrics",
	}
	brokenPanelCopy = emptyCopy{
		NoResults: "No broken panels found - every dashboard panel query resolves to a metric that exists in Prometheus head series or as a recording-rule output.",
		Subject:   "broken panels",
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

// renderEmpty writes the appropriate empty-state output for cfg.Output.
//
// totalCount is the number of findings that survived the --ignore filter
// (callers pass len(all) where `all` is post-Apply). When totalCount == 0
// either the analyzer produced no findings or every finding was dropped
// by --ignore - show copy.NoResults plus a doctor hint. The non-zero
// `ignored` footer disambiguates these two cases for the user.
//
// When totalCount > 0 but the slice handed to the renderer is empty,
// every finding was below minSev - show a filter hint with the
// severity distribution and an actionable `--min-severity` suggestion.
//
// JSON output ignores the empty-state copy entirely and emits the
// standard `{"findings": [], "summary": ...}` envelope so machine
// consumers see a stable wire contract.
//
// ignored is the number of findings dropped by --ignore-* patterns. For
// terminal output a footer line is appended when non-zero. For JSON the
// envelope carries `ignored_count` via RenderFindingsWithIgnored.
func renderEmpty(cfg *config.Config, w io.Writer, c emptyCopy,
	minSev findings.Severity, totalCount int, bySev map[findings.Severity]int,
	ignored int,
) error {
	switch cfg.Output {
	case "", "terminal":
		if totalCount == 0 {
			if _, err := fmt.Fprintln(w, c.NoResults+" Run `remetric doctor` to verify the connection."); err != nil {
				return err
			}
			return writeTerminalIgnored(w, ignored)
		}
		if _, err := fmt.Fprintf(w,
			"Filtered %d %s below %s severity: %s. Try `--min-severity low` to see them all.\n",
			totalCount, c.Subject, strings.ToLower(minSev.String()), formatSeverityCounts(bySev)); err != nil {
			return err
		}
		return writeTerminalIgnored(w, ignored)
	case "json":
		return outjson.New(w).RenderFindingsWithIgnored(nil, ignored)
	default:
		return fmt.Errorf("unsupported --output %q (want terminal|json)", cfg.Output)
	}
}

// formatSeverityCounts renders "3 low, 2 medium" from a tally map.
// Order: low → medium → high → critical. Zero counts skipped.
func formatSeverityCounts(bySev map[findings.Severity]int) string {
	order := []findings.Severity{
		findings.SeverityLow, findings.SeverityMedium,
		findings.SeverityHigh, findings.SeverityCritical,
	}
	parts := make([]string, 0, len(order))
	for _, s := range order {
		if n := bySev[s]; n > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", n, strings.ToLower(s.String())))
		}
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, ", ")
}
