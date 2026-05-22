// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

// Package dashboardhygiene flags Grafana dashboards whose panel queries
// reference Prometheus metric names that do not exist in the head
// series or in any recording-rule output. One finding per
// (dashboard, missing-metric) pair, severity Medium.
package dashboardhygiene

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"sort"

	"github.com/remetric-dev/remetric/internal/analyzers"
	"github.com/remetric-dev/remetric/internal/findings"
	"github.com/remetric-dev/remetric/internal/grafana"
	"github.com/remetric-dev/remetric/internal/prometheus"
	"github.com/remetric-dev/remetric/internal/promqlx"
)

// maxSamplePanelTitles is the upper bound on Evidence.SampleValues
// per broken-panel finding. Beyond this, panel titles are listed
// only in the Fix snippet, not as structured evidence.
const maxSamplePanelTitles = 5

// errRulesUnavailable signals that the rules endpoint is unreachable
// on the detected flavor (VictoriaMetrics without a vmalert client).
// Callers convert it to a Result warning rather than failing the
// analyzer. Matches the contract in internal/analyzers/unusedmetrics.
var errRulesUnavailable = errors.New("rules unavailable")

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

	var warnings []string
	if err := collectRecordingOutputs(ctx, d, exists); err != nil {
		if errors.Is(err, errRulesUnavailable) {
			warnings = append(warnings, "rules unavailable: VictoriaMetrics detected without --vmalert URL - recording rules ignored")
		} else {
			return analyzers.Result{}, err
		}
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
	sort.SliceStable(out, func(i, j int) bool { return findingLess(out[i], out[j]) })
	return analyzers.Result{Findings: out, Warnings: warnings}, nil
}

// collectRecordingOutputs adds the output name of every recording
// rule to exists. Prefers d.VMAlert over d.Prom when present. On
// VictoriaMetrics without a vmalert URL the sentinel errRulesUnavailable
// is returned so callers can degrade gracefully.
//
// Unlike unusedmetrics.collectRuleUsage, this helper does not extract
// metric names from rule expressions. The question this analyzer asks
// is "does this metric exist?", which is answered by the rule's output
// name, not by the metrics it consumes.
func collectRecordingOutputs(ctx context.Context, d analyzers.Deps, exists map[string]struct{}) error {
	client := d.Prom
	if d.VMAlert != nil {
		client = d.VMAlert
	}
	rules, err := client.Rules(ctx)
	if err != nil {
		if errors.Is(err, prometheus.ErrNotFound) {
			// 404 on /api/v1/rules can mean older VM builds or non-VM
			// gateways; trigger cached flavor detection so the next
			// check reads Flavor() reliably regardless of call order.
			// Same pattern as unusedmetrics.collectRuleUsage.
			_, _ = d.Prom.BuildInfo(ctx)
			if d.Prom.Flavor() == prometheus.FlavorVictoria {
				return errRulesUnavailable
			}
		}
		return fmt.Errorf("dashboardhygiene: rules: %w", err)
	}
	// VictoriaMetrics single-node serves /api/v1/rules at HTTP 200 with
	// empty or partial groups because recording rules live in vmalert.
	// Without --vmalert we can't trust this payload to be complete, so
	// short-circuit to the same sentinel the 404 path takes. Mirrors
	// the unusedmetrics.collectRuleUsage rationale (intentional dup).
	if d.VMAlert == nil {
		_, _ = d.Prom.BuildInfo(ctx)
		if d.Prom.Flavor() == prometheus.FlavorVictoria {
			return errRulesUnavailable
		}
	}
	for _, g := range rules.Groups {
		for _, r := range g.Rules {
			if r.Type == "recording" && r.Name != "" {
				exists[r.Name] = struct{}{}
			}
		}
	}
	return nil
}

// findingLess orders broken-panel findings deterministically by
// severity (desc), sample-count (desc), dashboard (asc), metric (asc).
// The Dashboard tiebreaker is required because two findings can share
// (severity, sample-count, metric) when different dashboards reference
// the same missing metric.
func findingLess(a, b findings.Finding) bool {
	if a.Severity != b.Severity {
		return a.Severity > b.Severity
	}
	na, nb := len(a.Evidence.SampleValues), len(b.Evidence.SampleValues)
	if na != nb {
		return na > nb
	}
	if a.Dashboard != b.Dashboard {
		return a.Dashboard < b.Dashboard
	}
	return a.Metric < b.Metric
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
func buildFinding(uid, missing string, agg dashAgg) findings.Finding {
	n := len(agg.panelTitles)
	sample := agg.panelTitles
	if n > maxSamplePanelTitles {
		sample = sample[:maxSamplePanelTitles]
	}
	return findings.Finding{
		ID:        "broken-panel:" + uid + ":" + missing,
		Severity:  findings.SeverityMedium,
		Category:  findings.CategoryDashboardHygiene,
		Class:     findings.ClassBrokenPanel,
		Title:     fmt.Sprintf("dashboard %q references missing metric %q", agg.dashTitle, missing),
		Metric:    missing,
		Dashboard: agg.dashTitle,
		Evidence: findings.Evidence{
			Description: fmt.Sprintf(
				"%d panel(s) in dashboard %q query %q which is not present in head series or recording-rule outputs",
				n, agg.dashTitle, missing),
			SampleValues: sample,
		},
		Fix: findings.Fix{
			Type:   "edit_dashboard",
			Config: buildFix(agg.dashTitle, missing, agg.dashURL, agg.panelTitles),
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
