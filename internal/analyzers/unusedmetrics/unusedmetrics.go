// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package unusedmetrics

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/remetric-dev/remetric/internal/analyzers"
	"github.com/remetric-dev/remetric/internal/findings"
	"github.com/remetric-dev/remetric/internal/grafana"
	"github.com/remetric-dev/remetric/internal/prometheus"
	"github.com/remetric-dev/remetric/internal/promqlx"
	"github.com/remetric-dev/remetric/internal/scoring"
)

// errRulesUnavailable signals that the rules endpoint is unreachable on the
// detected flavor (e.g., VictoriaMetrics without a vmalert client). It is a
// graceful-degradation sentinel: callers convert it to a Result warning
// rather than failing the analyzer.
var errRulesUnavailable = errors.New("rules unavailable")

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
func (a *Analyzer) Analyze(ctx context.Context, d analyzers.Deps) (analyzers.Result, error) {
	ingested, err := d.Prom.LabelValues(ctx, "__name__")
	if err != nil {
		return analyzers.Result{}, fmt.Errorf("unusedmetrics: ingested names: %w", err)
	}

	used := map[string]struct{}{}
	var warnings []string

	dashWarnings, err := collectDashboardUsage(ctx, d.Graf, used)
	if err != nil {
		return analyzers.Result{}, err
	}
	warnings = append(warnings, dashWarnings...)
	if err := collectRuleUsage(ctx, d, used); err != nil {
		if errors.Is(err, errRulesUnavailable) {
			warnings = append(warnings, "rules unavailable: VictoriaMetrics detected without --vmalert URL — recording rules ignored")
		} else {
			return analyzers.Result{}, err
		}
	}

	stats, err := d.Prom.TSDBStats(ctx, len(ingested))
	if err != nil {
		return analyzers.Result{}, fmt.Errorf("unusedmetrics: tsdb stats: %w", err)
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
			return analyzers.Result{}, err
		}
		out = append(out, f)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return out[i].Severity > out[j].Severity
		}
		return out[i].Evidence.SeriesCount > out[j].Evidence.SeriesCount
	})
	return analyzers.Result{Findings: out, Warnings: warnings}, nil
}

// collectDashboardUsage walks every Grafana dashboard and records the
// metric names referenced by Prometheus targets. It is a no-op when
// graf is nil. Per-dashboard fetch errors are collected and returned
// as warnings so a single broken dashboard does not abort the scan.
func collectDashboardUsage(ctx context.Context, graf *grafana.Client, used map[string]struct{}) ([]string, error) {
	if graf == nil {
		return nil, nil
	}
	refs, err := graf.Search(ctx)
	if err != nil {
		return nil, fmt.Errorf("unusedmetrics: grafana search: %w", err)
	}
	var warnings []string
	for _, ref := range refs {
		dash, err := graf.Dashboard(ctx, ref.UID)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("unusedmetrics: dashboard %q: %v", ref.UID, err))
			continue
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
	return warnings, nil
}

// collectRuleUsage records every metric referenced by an alerting or
// recording rule expression, plus the output name of each recording
// rule. When d.VMAlert is non-nil it is preferred over d.Prom — this
// covers the VictoriaMetrics topology where vmselect (the main client)
// does not serve /api/v1/rules but vmalert does.
//
// On VictoriaMetrics without a vmalert URL we surface errRulesUnavailable
// (a non-fatal sentinel) so callers can degrade gracefully. Two paths
// reach that sentinel: the rule call 404s (older VM builds, or non-VM
// gateways), or it returns 200 with empty/partial groups (real VM
// single-node behavior — vmselect serves the endpoint but recording
// rules live in vmalert which we don't have a URL for). We cannot
// distinguish "no rules configured anywhere" from "rules live in
// vmalert we can't see", so we warn either way.
func collectRuleUsage(ctx context.Context, d analyzers.Deps, used map[string]struct{}) error {
	client := d.Prom
	if d.VMAlert != nil {
		client = d.VMAlert
	}
	rules, err := client.Rules(ctx)
	if err != nil {
		if errors.Is(err, prometheus.ErrNotFound) {
			// Trigger flavor detection so the graceful-degradation branch
			// can read Flavor() reliably regardless of step ordering in
			// Analyze. BuildInfo is cached after the first call.
			_, _ = d.Prom.BuildInfo(ctx)
			if d.Prom.Flavor() == prometheus.FlavorVictoria {
				return errRulesUnavailable
			}
		}
		return fmt.Errorf("unusedmetrics: rules: %w", err)
	}
	// Real-world VictoriaMetrics single-node serves /api/v1/rules at HTTP
	// 200 with `{"data":{"groups":[]}}` (or a partial view), because
	// recording rules are owned by vmalert — a separate process. Without
	// a --vmalert URL we cannot trust this payload to be complete, so
	// short-circuit to the same warning the 404 path takes.
	if d.VMAlert == nil {
		_, _ = d.Prom.BuildInfo(ctx) // trigger flavor detection (cached)
		if d.Prom.Flavor() == prometheus.FlavorVictoria {
			return errRulesUnavailable
		}
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
