// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

// Package analyzers defines the common contract for all analyzers.
package analyzers

import (
	"context"
	"log/slog"
	"time"

	"github.com/remetric-dev/remetric/internal/findings"
	"github.com/remetric-dev/remetric/internal/grafana"
	"github.com/remetric-dev/remetric/internal/prometheus"
)

// Deps groups the dependencies passed to an analyzer at Analyze time.
type Deps struct {
	Prom *prometheus.Client
	// Graf may be nil - analyzers that need Grafana skip when absent.
	Graf *grafana.Client
	// VMAlert is an optional second client pointed at vmalert for /api/v1/rules.
	// When nil, analyzers fall back to Prom for rule queries.
	VMAlert *prometheus.Client
	Logger  *slog.Logger
	Limits  Limits
}

// Limits caps the work an analyzer performs.
type Limits struct {
	TopMetrics     int
	SampleSize     int
	PerCallTimeout time.Duration
}

// DefaultLimits returns the Phase 1 defaults.
func DefaultLimits() Limits {
	return Limits{
		TopMetrics:     20,
		SampleSize:     5,
		PerCallTimeout: 30 * time.Second,
	}
}

// Result is the analyzer output: zero or more findings plus
// non-fatal degradation warnings (e.g., "vmalert URL not configured").
type Result struct {
	Findings []findings.Finding
	Warnings []string
}

// Analyzer produces findings.
type Analyzer interface {
	Name() string
	Analyze(ctx context.Context, d Deps) (Result, error)
}
