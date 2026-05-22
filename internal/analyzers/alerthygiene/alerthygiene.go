// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

// Package alerthygiene flags alerting rules that never fire or fire
// continuously, by inspecting the ALERTS series via query_range.
package alerthygiene

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"sync/atomic"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/remetric-dev/remetric/internal/analyzers"
	"github.com/remetric-dev/remetric/internal/findings"
	prom "github.com/remetric-dev/remetric/internal/prometheus"
)

// alwaysFiringThreshold is the share of evaluation steps with
// alertstate="firing" above which an alert is flagged as always-firing.
const alwaysFiringThreshold = 0.95

// classification is the analyzer's verdict on a single alert.
type classification int

const (
	classNone classification = iota
	classNeverFired
	classAlwaysFiring
)

// classify inspects the ALERTS-series result of one alert over a window
// of totalSteps and returns the classification plus the firing ratio
// (samples with alertstate="firing" divided by totalSteps).
func classify(series []prom.Series, totalSteps int) (classification, float64) {
	if totalSteps <= 0 {
		return classNone, 0
	}
	var firing int
	for _, s := range series {
		if s.Metric["alertstate"] != "firing" {
			continue
		}
		firing += len(s.Values)
	}
	if firing == 0 {
		return classNeverFired, 0
	}
	ratio := float64(firing) / float64(totalSteps)
	if ratio >= alwaysFiringThreshold {
		return classAlwaysFiring, ratio
	}
	return classNone, ratio
}

// computeTotalSteps returns ceil(lookback / step) as a step count.
func computeTotalSteps(lookback, step time.Duration) int {
	if step <= 0 || lookback <= 0 {
		return 0
	}
	return int(math.Ceil(float64(lookback) / float64(step)))
}

// Config tunes the analyzer. Zero values pick sensible defaults.
type Config struct {
	// Lookback is how far back to scan ALERTS series. Default 168h.
	Lookback time.Duration
	// Step is the query_range step. Default 1h.
	Step time.Duration
	// Now is the clock injection seam; default time.Now.
	Now func() time.Time
}

func (c Config) withDefaults() Config {
	if c.Lookback <= 0 {
		c.Lookback = 168 * time.Hour
	}
	if c.Step <= 0 {
		c.Step = time.Hour
	}
	if c.Now == nil {
		c.Now = time.Now
	}
	return c
}

// Analyzer flags alerting rules that never fire or fire continuously.
type Analyzer struct {
	cfg Config
}

// New constructs an Analyzer with the provided config.
func New(cfg Config) *Analyzer { return &Analyzer{cfg: cfg.withDefaults()} }

// Name implements analyzers.Analyzer.
func (a *Analyzer) Name() string { return "alerthygiene" }

// fetchRules retrieves rule groups from the configured client, returning a
// non-empty warning string instead of rules when the backend can't be served:
//
//   - HTTP 404 on /api/v1/rules → generic "endpoint returned 404" warning.
//   - VictoriaMetrics flavor without a --vmalert URL → VM-specific warning.
//     Real-world VM single-node serves the endpoint at HTTP 200 with empty
//     groups (recording/alerting rules live in vmalert, a separate process),
//     so we must check the flavor on the success path too, not just on 404.
//
// A non-nil error is reserved for unexpected transport failures.
func fetchRules(ctx context.Context, d analyzers.Deps) (*prom.Rules, string, error) {
	rulesClient := d.Prom
	if d.VMAlert != nil {
		rulesClient = d.VMAlert
	}
	rules, err := rulesClient.Rules(ctx)
	notFound := false
	if err != nil {
		if !errors.Is(err, prom.ErrNotFound) {
			return nil, "", fmt.Errorf("alerthygiene: rules: %w", err)
		}
		notFound = true
	}
	if d.VMAlert == nil {
		_, _ = d.Prom.BuildInfo(ctx) // trigger flavor detection (cached)
		if d.Prom.Flavor() == prom.FlavorVictoria {
			return nil, "alerthygiene: rules unavailable: VictoriaMetrics detected without --vmalert URL", nil
		}
	}
	if notFound {
		return nil, "alerthygiene: rules unavailable - endpoint returned 404", nil
	}
	return rules, "", nil
}

// flatRule pairs an alerting rule with its enclosing group for downstream
// reporting after the parallel classify fan-out flattens the rule tree.
type flatRule struct {
	group prom.RuleGroup
	rule  prom.Rule
}

// flattenAlertingRules returns one flatRule per alerting rule across all
// groups, dropping recording rules.
func flattenAlertingRules(rules *prom.Rules) []flatRule {
	var flat []flatRule
	for _, g := range rules.Groups {
		for _, r := range g.Rules {
			if r.Type != "alerting" {
				continue
			}
			flat = append(flat, flatRule{group: g, rule: r})
		}
	}
	return flat
}

// classifyResult carries one classified alert from a worker goroutine back
// to the main Analyze loop for sorting and finding construction.
type classifyResult struct {
	flat  flatRule
	cls   classification
	ratio float64
}

// Analyze implements analyzers.Analyzer.
func (a *Analyzer) Analyze(ctx context.Context, d analyzers.Deps) (analyzers.Result, error) {
	rules, warning, err := fetchRules(ctx, d)
	if err != nil {
		return analyzers.Result{}, err
	}
	if warning != "" {
		return analyzers.Result{Warnings: []string{warning}}, nil
	}

	totalSteps := computeTotalSteps(a.cfg.Lookback, a.cfg.Step)
	end := a.cfg.Now()
	start := end.Add(-a.cfg.Lookback)
	flat := flattenAlertingRules(rules)

	ch := make(chan classifyResult, len(flat))
	var skipped atomic.Int32

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(d.Prom.MaxInFlight())
	for _, fr := range flat {
		fr := fr
		g.Go(func() error {
			return a.classifyOne(ctx, d, fr, start, end, totalSteps, ch, &skipped)
		})
	}
	waitErr := g.Wait()
	close(ch)

	if waitErr != nil {
		if errors.Is(waitErr, prom.ErrNotSupported) {
			return analyzers.Result{
				Warnings: []string{"alerthygiene: query_range not supported on backend - skipping analyzer"},
			}, nil
		}
		return analyzers.Result{}, waitErr
	}

	var out []findings.Finding
	for r := range ch {
		out = append(out, buildFinding(r.flat.rule, r.flat.group, r.cls, r.ratio, a.cfg.Lookback, totalSteps))
	}

	var warnings []string
	if s := skipped.Load(); s > 0 {
		warnings = append(warnings, fmt.Sprintf("alerthygiene: skipped %d alerts due to query failures", s))
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Severity != out[j].Severity {
			return out[i].Severity > out[j].Severity
		}
		return out[i].Title < out[j].Title
	})
	return analyzers.Result{Findings: out, Warnings: warnings}, nil
}

// classifyOne queries ALERTS for a single alerting rule and either pushes a
// classifyResult onto ch, increments skipped on transient failure, or returns
// prom.ErrNotSupported to abort the whole errgroup.
func (a *Analyzer) classifyOne(
	ctx context.Context,
	d analyzers.Deps,
	fr flatRule,
	start, end time.Time,
	totalSteps int,
	ch chan<- classifyResult,
	skipped *atomic.Int32,
) error {
	expr := fmt.Sprintf(`ALERTS{alertname=%q}`, fr.rule.Name)
	res, err := d.Prom.QueryRange(ctx, expr, start, end, a.cfg.Step)
	if err != nil {
		if errors.Is(err, prom.ErrNotSupported) {
			return err
		}
		if d.Logger != nil {
			d.Logger.Debug("alerthygiene: query_range failed", "alert", fr.rule.Name, "err", err)
		}
		skipped.Add(1)
		return nil
	}
	cls, ratio := classify(res.Result, totalSteps)
	if cls == classNone {
		return nil
	}
	ch <- classifyResult{flat: fr, cls: cls, ratio: ratio}
	return nil
}

func buildFinding(r prom.Rule, g prom.RuleGroup, cls classification, ratio float64, lookback time.Duration, steps int) findings.Finding {
	switch cls {
	case classNeverFired:
		return findings.Finding{
			ID:       "alert_hygiene/never_fired",
			Severity: findings.SeverityMedium,
			Category: findings.CategoryAlertHygiene,
			Class:    findings.ClassNeverFiringAlert,
			Alert:    r.Name,
			Title:    fmt.Sprintf("Alert %s never fired in %s", r.Name, lookback),
			Evidence: findings.Evidence{
				Description: fmt.Sprintf("no firing samples in %s lookback (alertname=%s, file=%s, group=%s)",
					lookback, r.Name, g.File, g.Name),
			},
			Fix: findings.Fix{
				Type: "rule_change",
				Config: fmt.Sprintf("# In %s, group %s: consider removing or relaxing this alert\n- alert: %s\n  expr: %s",
					g.File, g.Name, r.Name, r.Query),
			},
		}
	case classAlwaysFiring:
		// Clamp ratio to 1.0 in case overlapping firing series push it above.
		clamped := math.Min(ratio, 1.0)
		firing := int(float64(steps) * clamped)
		return findings.Finding{
			ID:       "alert_hygiene/always_firing",
			Severity: findings.SeverityCritical,
			Category: findings.CategoryAlertHygiene,
			Class:    findings.ClassAlwaysFiringAlert,
			Alert:    r.Name,
			Title:    fmt.Sprintf("Alert %s firing %.1f%% of window (%d/%d steps)", r.Name, clamped*100, firing, steps),
			Evidence: findings.Evidence{
				Description: fmt.Sprintf("firing %.1f%% of %s lookback - likely broken threshold (alertname=%s, file=%s, group=%s)",
					clamped*100, lookback, r.Name, g.File, g.Name),
			},
			Fix: findings.Fix{
				Type: "rule_change",
				Config: fmt.Sprintf("# In %s, group %s: review threshold - alert is permanently firing\n- alert: %s\n  expr: %s",
					g.File, g.Name, r.Name, r.Query),
			},
		}
	case classNone:
		// no-op
	}
	return findings.Finding{}
}
