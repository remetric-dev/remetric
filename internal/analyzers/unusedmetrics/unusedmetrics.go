// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package unusedmetrics

import (
	"context"
	"fmt"
	"sort"

	"github.com/remetric-dev/remetric/internal/analyzers"
	"github.com/remetric-dev/remetric/internal/findings"
	"github.com/remetric-dev/remetric/internal/grafana"
	"github.com/remetric-dev/remetric/internal/prometheus"
	"github.com/remetric-dev/remetric/internal/promqlx"
	"github.com/remetric-dev/remetric/internal/scoring"
)

// Analyzer flags ingested metrics that no Grafana dashboard, alert
// rule, or recording rule references.
type Analyzer struct{}

// New constructs an Analyzer with default settings.
func New() *Analyzer { return &Analyzer{} }

// Name implements analyzers.Analyzer.
func (a *Analyzer) Name() string { return "unusedmetrics" }

// Analyze implements analyzers.Analyzer.
//
// When Deps.Graf is nil, dashboard-based usage signal is skipped;
// only rules + recording-output names are considered "used".
func (a *Analyzer) Analyze(ctx context.Context, d analyzers.Deps) ([]findings.Finding, error) {
	ingested, err := d.Prom.LabelValues(ctx, "__name__")
	if err != nil {
		return nil, fmt.Errorf("unusedmetrics: ingested names: %w", err)
	}

	// Used set: dashboard queries + alert exprs + recording-rule exprs + recording output names.
	used := map[string]struct{}{}
	if err := collectDashboardUsage(ctx, d.Graf, used); err != nil {
		return nil, err
	}
	if err := collectRuleUsage(ctx, d.Prom, used); err != nil {
		return nil, err
	}

	// Map metric name → head series count via TSDB stats.
	stats, err := d.Prom.TSDBStats(ctx, len(ingested))
	if err != nil {
		return nil, fmt.Errorf("unusedmetrics: tsdb stats: %w", err)
	}
	seriesByMetric := make(map[string]int64, len(stats.SeriesCountByMetricName))
	for _, m := range stats.SeriesCountByMetricName {
		seriesByMetric[m.Name] = m.Value
	}

	out := make([]findings.Finding, 0, len(ingested))
	for _, name := range ingested {
		if _, ok := used[name]; ok {
			continue
		}
		f, err := buildFinding(name, seriesByMetric[name])
		if err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return out[i].Severity > out[j].Severity
		}
		return out[i].Evidence.SeriesCount > out[j].Evidence.SeriesCount
	})
	return out, nil
}

// collectDashboardUsage walks every Grafana dashboard and records the
// metric names referenced by Prometheus targets. It is a no-op when
// graf is nil.
func collectDashboardUsage(ctx context.Context, graf *grafana.Client, used map[string]struct{}) error {
	if graf == nil {
		return nil
	}
	refs, err := graf.Search(ctx)
	if err != nil {
		return fmt.Errorf("unusedmetrics: grafana search: %w", err)
	}
	for _, ref := range refs {
		dash, err := graf.Dashboard(ctx, ref.UID)
		if err != nil {
			// TODO(phase4): aggregate per-dashboard errors instead of fail-fast.
			return fmt.Errorf("unusedmetrics: dashboard %q: %w", ref.UID, err)
		}
		for _, q := range dash.Queries() {
			m, err := promqlx.ExtractFromQuery(q)
			if err != nil {
				continue // unparseable dashboard query is best-effort skipped
			}
			for name := range m {
				used[name] = struct{}{}
			}
		}
	}
	return nil
}

// collectRuleUsage records every metric referenced by an alerting or
// recording rule expression, plus the output name of each recording
// rule.
func collectRuleUsage(ctx context.Context, prom *prometheus.Client, used map[string]struct{}) error {
	rules, err := prom.Rules(ctx)
	if err != nil {
		return fmt.Errorf("unusedmetrics: rules: %w", err)
	}
	for _, g := range rules.Groups {
		for _, r := range g.Rules {
			if m, err := promqlx.ExtractFromQuery(r.Query); err == nil {
				for name := range m {
					used[name] = struct{}{}
				}
			}
			if r.Type == "recording" && r.Name != "" {
				used[r.Name] = struct{}{}
			}
		}
	}
	return nil
}

// buildFinding renders the Finding for one unused metric with the
// given head series count.
func buildFinding(name string, series int64) (findings.Finding, error) {
	fixCfg, err := RenderFix(name)
	if err != nil {
		return findings.Finding{}, fmt.Errorf("unusedmetrics: render fix: %w", err)
	}
	return findings.Finding{
		ID:       "unused-" + name,
		Severity: scoring.UnusedMetricSeverity(series),
		Category: findings.CategoryUnusedMetrics,
		Title:    fmt.Sprintf("metric %q is not referenced by any dashboard, alert, or recording rule", name),
		Metric:   name,
		Evidence: findings.Evidence{
			SeriesCount: series,
			Description: fmt.Sprintf("%s has %d series and no observed user", name, series),
		},
		Fix: findings.Fix{
			Type:   "drop_metric",
			Config: fixCfg,
		},
		Impact: findings.Impact{
			SeriesReduction:  series,
			Percentage:       100,
			EstimationMethod: "full_drop",
		},
	}, nil
}
