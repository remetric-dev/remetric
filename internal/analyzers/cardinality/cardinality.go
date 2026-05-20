// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

// Package cardinality identifies high-cardinality metrics and attributes them
// to the label that drives the explosion.
package cardinality

import (
	"context"
	"fmt"
	"sort"

	"github.com/remetric-dev/remetric/internal/analyzers"
	"github.com/remetric-dev/remetric/internal/findings"
	"github.com/remetric-dev/remetric/internal/scoring"
)

// Analyzer identifies high-cardinality metrics and attributes them
// to the label that contributes most uniqueness.
type Analyzer struct{}

// New constructs the analyzer with default settings.
func New() *Analyzer { return &Analyzer{} }

// Name implements analyzers.Analyzer.
func (a *Analyzer) Name() string { return "cardinality" }

// Analyze implements analyzers.Analyzer.
func (a *Analyzer) Analyze(ctx context.Context, d analyzers.Deps) ([]findings.Finding, error) {
	stats, err := d.Prom.TSDBStats(ctx, d.Limits.TopMetrics)
	if err != nil {
		return nil, fmt.Errorf("cardinality: tsdb stats: %w", err)
	}
	head := stats.HeadStats.NumSeries

	var out []findings.Finding
	for _, m := range stats.SeriesCountByMetricName {
		labels, err := d.Prom.LabelNamesForMetric(ctx, m.Name)
		if err != nil {
			// TODO(phase2): collect per-metric errors and continue rather than fail-fast.
			return nil, fmt.Errorf("cardinality: labels for %q: %w", m.Name, err)
		}

		// TODO(phase2): parallelise per-label LabelValues calls bounded by the client semaphore.
		topLabel, topValues, err := worstLabel(ctx, d, m.Name, labels)
		if err != nil {
			return nil, err
		}
		if topLabel == "" {
			continue
		}

		sev := scoring.CardinalitySeverity(m.Value, head)
		if sev == findings.SeverityLow {
			continue
		}

		fixCfg, err := renderFix(m.Name, topLabel)
		if err != nil {
			return nil, fmt.Errorf("cardinality: render fix: %w", err)
		}

		card := max(int64(len(topValues)), 1)
		reduction := m.Value - m.Value/card
		pct := 0.0
		if m.Value > 0 {
			pct = float64(reduction) / float64(m.Value) * 100
		}

		out = append(out, findings.Finding{
			ID:       fmt.Sprintf("card-%s-%s", m.Name, topLabel),
			Severity: sev,
			Category: findings.CategoryCardinality,
			Title:    fmt.Sprintf("high cardinality in %s due to label %q", m.Name, topLabel),
			Metric:   m.Name,
			Evidence: findings.Evidence{
				Label:        topLabel,
				UniqueValues: len(topValues),
				SampleValues: sample(topValues, d.Limits.SampleSize),
				SeriesCount:  m.Value,
				Description:  fmt.Sprintf("%s has %d unique values inside %s", topLabel, len(topValues), m.Name),
			},
			Fix: findings.Fix{
				Type:   "scrape_config",
				Config: fixCfg,
			},
			Impact: findings.Impact{
				SeriesReduction:  reduction,
				Percentage:       pct,
				EstimationMethod: "labeldrop_upper_bound",
			},
		})
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return out[i].Severity > out[j].Severity
		}
		return out[i].Evidence.SeriesCount > out[j].Evidence.SeriesCount
	})

	return out, nil
}

func worstLabel(ctx context.Context, d analyzers.Deps, metric string, labels []string) (string, []string, error) {
	var bestLabel string
	var bestValues []string
	for _, lbl := range labels {
		matcher := fmt.Sprintf(`{__name__="%s"}`, metric)
		vals, err := d.Prom.LabelValues(ctx, lbl, matcher)
		if err != nil {
			return "", nil, fmt.Errorf("cardinality: values for %q[%q]: %w", metric, lbl, err)
		}
		if len(vals) > len(bestValues) {
			bestLabel = lbl
			bestValues = vals
		}
	}
	return bestLabel, bestValues, nil
}

func sample(values []string, n int) []string {
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
