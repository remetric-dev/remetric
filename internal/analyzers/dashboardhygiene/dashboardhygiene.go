// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

// Package dashboardhygiene flags Grafana dashboards whose panel queries
// reference Prometheus metric names that do not exist in the head
// series or in any recording-rule output. One finding per
// (dashboard, missing-metric) pair, severity Medium.
package dashboardhygiene

import (
	"context"

	"github.com/remetric-dev/remetric/internal/analyzers"
)

// Analyzer is the dashboard-hygiene analyzer (broken panels).
type Analyzer struct{}

// New constructs an Analyzer with default settings.
func New() *Analyzer { return &Analyzer{} }

// Name implements analyzers.Analyzer.
func (a *Analyzer) Name() string { return "dashboardhygiene" }

// Analyze implements analyzers.Analyzer. Without a Grafana client
// the analyzer cannot operate: returns a single warning, no findings,
// no error (consistent with unusedmetrics).
func (a *Analyzer) Analyze(_ context.Context, d analyzers.Deps) (analyzers.Result, error) {
	if d.Graf == nil {
		return analyzers.Result{Warnings: []string{"grafana not configured: skipped"}}, nil
	}
	// Implementation continues in Task 6+.
	return analyzers.Result{}, nil
}
