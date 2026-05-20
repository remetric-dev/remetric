// Package analyzers defines the common contract for all analyzers.
package analyzers

import (
	"context"
	"log/slog"
	"time"

	"github.com/remetric-dev/remetric/internal/findings"
	"github.com/remetric-dev/remetric/internal/prometheus"
)

// Deps groups the dependencies passed to an analyzer at Analyze time.
type Deps struct {
	Prom   *prometheus.Client
	Logger *slog.Logger
	Limits Limits
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

// Analyzer produces findings.
type Analyzer interface {
	Name() string
	Analyze(ctx context.Context, d Deps) ([]findings.Finding, error)
}
