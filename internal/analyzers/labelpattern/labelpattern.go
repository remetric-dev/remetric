// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package labelpattern

import (
	"context"
	"fmt"
	"regexp"
	"sort"

	"github.com/remetric-dev/remetric/internal/analyzers"
	"github.com/remetric-dev/remetric/internal/findings"
	"github.com/remetric-dev/remetric/internal/scoring"
)

// Analyzer flags labels whose names match well-known unbounded-identifier
// patterns and whose cardinality is non-trivial.
type Analyzer struct {
	patterns []*regexp.Regexp
}

// New constructs an Analyzer with the default suspicious patterns.
func New() *Analyzer {
	return &Analyzer{patterns: DefaultPatterns()}
}

// NewWithPatterns constructs an Analyzer with caller-supplied patterns.
// Returns the first compile error if any source is invalid.
func NewWithPatterns(sources []string) (*Analyzer, error) {
	pats, err := CompilePatterns(sources)
	if err != nil {
		return nil, err
	}
	return &Analyzer{patterns: pats}, nil
}

// Name implements analyzers.Analyzer.
func (a *Analyzer) Name() string { return "labelpattern" }

// Analyze implements analyzers.Analyzer.
func (a *Analyzer) Analyze(ctx context.Context, d analyzers.Deps) (analyzers.Result, error) {
	stats, err := d.Prom.TSDBStats(ctx, d.Limits.TopMetrics)
	if err != nil {
		return analyzers.Result{}, fmt.Errorf("labelpattern: tsdb stats: %w", err)
	}

	seriesByMetric := make(map[string]int64, len(stats.SeriesCountByMetricName))
	for _, m := range stats.SeriesCountByMetricName {
		seriesByMetric[m.Name] = m.Value
	}

	var out []findings.Finding
	for _, lbl := range stats.LabelValueCountByLabelName {
		if IsBounded(lbl.Name) {
			continue
		}
		if !MatchesAny(a.patterns, lbl.Name) {
			continue
		}

		// TODO(phase3): heuristic verification that sampled values look
		// unbounded (variance in length, no obvious enum).

		samples, err := d.Prom.LabelValues(ctx, lbl.Name)
		if err != nil {
			return analyzers.Result{}, fmt.Errorf("labelpattern: sample %q: %w", lbl.Name, err)
		}

		// TODO(phase3): aggregate per-label errors instead of fail-fast.
		metrics, err := d.Prom.MetricNamesWithLabel(ctx, lbl.Name)
		if err != nil {
			return analyzers.Result{}, fmt.Errorf("labelpattern: metrics for %q: %w", lbl.Name, err)
		}

		var seriesAffected int64
		for _, m := range metrics {
			seriesAffected += seriesByMetric[m]
		}

		uniq := int(lbl.Value)
		sev := scoring.LabelPatternSeverity(uniq)
		impact := EstimateImpact(seriesAffected, uniq)

		fixCfg, err := RenderFix(lbl.Name, metrics)
		if err != nil {
			return analyzers.Result{}, fmt.Errorf("labelpattern: render fix: %w", err)
		}

		out = append(out, findings.Finding{
			ID:       "label-" + lbl.Name,
			Severity: sev,
			Category: findings.CategoryLabelPatterns,
			Title:    fmt.Sprintf("suspicious label %q with %d unique values across %d metrics", lbl.Name, uniq, len(metrics)),
			Evidence: findings.Evidence{
				Label:        lbl.Name,
				UniqueValues: uniq,
				SampleValues: sampleValues(samples, d.Limits.SampleSize),
				SeriesCount:  seriesAffected,
				Description:  fmt.Sprintf("%s has %d unique values across %d metrics", lbl.Name, uniq, len(metrics)),
			},
			Fix: findings.Fix{
				Type:   "drop_label",
				Config: fixCfg,
			},
			Impact: impact,
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return out[i].Severity > out[j].Severity
		}
		return out[i].Evidence.UniqueValues > out[j].Evidence.UniqueValues
	})

	return analyzers.Result{Findings: out}, nil
}

func sampleValues(values []string, n int) []string {
	if n <= 0 || len(values) == 0 {
		return nil
	}
	if n > len(values) {
		n = len(values)
	}
	out := make([]string, n)
	copy(out, values[:n])
	return out
}
