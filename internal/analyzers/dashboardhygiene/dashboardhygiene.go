// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

// Package dashboardhygiene flags Grafana dashboards whose panel queries
// reference Prometheus metric names that do not exist in the head
// series or in any recording-rule output. One finding per
// (dashboard, missing-metric) pair, severity Medium.
package dashboardhygiene

import (
	"context"
	"fmt"
	"net/url"
	"sort"

	"github.com/remetric-dev/remetric/internal/analyzers"
	"github.com/remetric-dev/remetric/internal/findings"
	"github.com/remetric-dev/remetric/internal/grafana"
	"github.com/remetric-dev/remetric/internal/promqlx"
)

// Analyzer is the dashboard-hygiene analyzer (broken panels).
type Analyzer struct{}

// New constructs an Analyzer with default settings.
func New() *Analyzer { return &Analyzer{} }

// Name implements analyzers.Analyzer.
func (a *Analyzer) Name() string { return "dashboardhygiene" }

// dashAgg accumulates per-(dashboard, missing-metric) state during
// the dashboard walk: which panels referenced the metric (titles
// preserved in walk order), the dashboard's title, and an absolute
// URL into Grafana for the Fix snippet.
type dashAgg struct {
	dashTitle   string
	dashURL     string
	panelTitles []string
}

// Analyze implements analyzers.Analyzer.
func (a *Analyzer) Analyze(ctx context.Context, d analyzers.Deps) (analyzers.Result, error) {
	if d.Graf == nil {
		return analyzers.Result{Warnings: []string{"grafana not configured: skipped"}}, nil
	}

	ingested, err := d.Prom.LabelValues(ctx, "__name__")
	if err != nil {
		return analyzers.Result{}, fmt.Errorf("dashboardhygiene: ingested names: %w", err)
	}
	exists := make(map[string]struct{}, len(ingested))
	for _, n := range ingested {
		exists[n] = struct{}{}
	}

	refs, err := d.Graf.Search(ctx)
	if err != nil {
		return analyzers.Result{}, fmt.Errorf("dashboardhygiene: grafana search: %w", err)
	}

	type key struct {
		uid    string
		metric string
	}
	groups := map[key]*dashAgg{}
	var warnings []string

	for _, ref := range refs {
		dash, err := d.Graf.Dashboard(ctx, ref.UID)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("dashboardhygiene: dashboard %q: %v", ref.UID, err))
			continue
		}
		dashURL := absoluteDashboardURL(d.Graf, ref.URL)
		for _, pt := range dash.PanelTargets() {
			names, err := promqlx.ExtractFromQuery(pt.Expr)
			if err != nil {
				continue
			}
			for name := range names {
				if _, ok := exists[name]; ok {
					continue
				}
				k := key{uid: ref.UID, metric: name}
				agg := groups[k]
				if agg == nil {
					agg = &dashAgg{dashTitle: dash.Title, dashURL: dashURL}
					groups[k] = agg
				}
				agg.panelTitles = append(agg.panelTitles, pt.PanelTitle)
			}
		}
	}

	out := make([]findings.Finding, 0, len(groups))
	for k, agg := range groups {
		out = append(out, buildFinding(k.uid, k.metric, *agg))
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return out[i].Severity > out[j].Severity
		}
		ni, nj := len(out[i].Evidence.SampleValues), len(out[j].Evidence.SampleValues)
		if ni != nj {
			return ni > nj
		}
		return out[i].Metric < out[j].Metric
	})
	return analyzers.Result{Findings: out, Warnings: warnings}, nil
}

// absoluteDashboardURL joins the Grafana base URL with the relative
// path from /api/search. Empty rel returns empty string.
func absoluteDashboardURL(c *grafana.Client, rel string) string {
	if rel == "" || c == nil {
		return ""
	}
	base := c.BaseURL()
	if base == nil {
		return ""
	}
	relURL, err := url.Parse(rel)
	if err != nil {
		return ""
	}
	return base.ResolveReference(relURL).String()
}

// buildFinding renders one Finding for a (dashboard, missing-metric) pair.
func buildFinding(uid, missing string, a dashAgg) findings.Finding {
	n := len(a.panelTitles)
	sample := a.panelTitles
	if n > 5 {
		sample = sample[:5]
	}
	return findings.Finding{
		ID:        "broken-panel:" + uid + ":" + missing,
		Severity:  findings.SeverityMedium,
		Category:  findings.CategoryDashboardHygiene,
		Class:     findings.ClassBrokenPanel,
		Title:     fmt.Sprintf("dashboard %q references missing metric %q", a.dashTitle, missing),
		Metric:    missing,
		Dashboard: a.dashTitle,
		Evidence: findings.Evidence{
			Description: fmt.Sprintf(
				"%d panel(s) in dashboard %q query %q which is not present in head series or recording-rule outputs",
				n, a.dashTitle, missing),
			SampleValues: sample,
		},
		Fix: findings.Fix{
			Type:   "edit_dashboard",
			Config: buildFix(a.dashTitle, missing, a.dashURL, a.panelTitles),
		},
		Impact: findings.Impact{
			EstimationMethod: "broken_panel",
		},
	}
}

// buildFix returns a paste-ready instruction block. Full implementation
// comes in Task 11 (fix.go). For now the stub returns an empty string
// so happy-path tests can pass without locking in the snippet format.
func buildFix(_, _, _ string, _ []string) string {
	return ""
}
