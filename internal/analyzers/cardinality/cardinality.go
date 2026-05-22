// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

// Package cardinality identifies high-cardinality metrics and attributes them
// to the label that drives the explosion.
package cardinality

import (
	"context"
	"fmt"
	"sort"
	"sync"

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
func (a *Analyzer) Analyze(ctx context.Context, d analyzers.Deps) (analyzers.Result, error) {
	stats, err := d.Prom.TSDBStats(ctx, d.Limits.TopMetrics)
	if err != nil {
		return analyzers.Result{}, fmt.Errorf("cardinality: tsdb stats: %w", err)
	}
	head := stats.HeadStats.NumSeries

	var out []findings.Finding
	var warnings []string
	for _, m := range stats.SeriesCountByMetricName {
		labels, err := d.Prom.LabelNamesForMetric(ctx, m.Name)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("cardinality: labels for %q: %v", m.Name, err))
			continue
		}

		topLabel, topValues, labelWarnings := worstLabel(ctx, d, m.Name, labels)
		warnings = append(warnings, labelWarnings...)
		if topLabel == "" {
			continue
		}

		sev := scoring.CardinalitySeverity(m.Value, head)
		if sev == findings.SeverityLow {
			continue
		}

		fixCfg, err := renderFix(m.Name, topLabel)
		if err != nil {
			return analyzers.Result{}, fmt.Errorf("cardinality: render fix: %w", err)
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
			Class:    findings.ClassHotLabel,
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

	return analyzers.Result{Findings: out, Warnings: warnings}, nil
}

// worstLabel returns the label with the most unique values for metric,
// querying every label in parallel. The prometheus.Client.maxInFlight
// semaphore bounds actual HTTP concurrency. Per-label errors are
// collected as warning strings; if every label fails, returns ("", nil,
// warnings) and the caller skips the metric.
//
// Ties on cardinality break by lexicographic label name (smallest
// wins) to keep output deterministic across runs.
func worstLabel(ctx context.Context, d analyzers.Deps, metric string, labels []string) (topLabel string, topValues []string, warnings []string) {
	type result struct {
		label string
		vals  []string
		err   error
	}
	ch := make(chan result, len(labels))
	var wg sync.WaitGroup
	for _, lbl := range labels {
		wg.Add(1)
		go func(lbl string) {
			defer wg.Done()
			matcher := fmt.Sprintf(`{__name__="%s"}`, metric)
			vals, err := d.Prom.LabelValues(ctx, lbl, matcher)
			ch <- result{label: lbl, vals: vals, err: err}
		}(lbl)
	}
	wg.Wait()
	close(ch)

	type success struct {
		label string
		vals  []string
	}
	var successes []success
	for r := range ch {
		if r.err != nil {
			warnings = append(warnings, fmt.Sprintf("cardinality: values for %q[%q]: %v", metric, r.label, r.err))
			continue
		}
		successes = append(successes, success{label: r.label, vals: r.vals})
	}
	sort.Slice(successes, func(i, j int) bool {
		if len(successes[i].vals) != len(successes[j].vals) {
			return len(successes[i].vals) > len(successes[j].vals)
		}
		return successes[i].label < successes[j].label
	})
	sort.Strings(warnings)
	if len(successes) > 0 {
		topLabel = successes[0].label
		topValues = successes[0].vals
	}
	return topLabel, topValues, warnings
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
