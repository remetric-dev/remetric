// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package dashboardhygiene

import (
	"context"
	"log/slog"
	"testing"

	"github.com/remetric-dev/remetric/internal/analyzers"
)

func TestAnalyzer_Name(t *testing.T) {
	if got := New().Name(); got != "dashboardhygiene" {
		t.Errorf("Name = %q, want %q", got, "dashboardhygiene")
	}
}

func TestAnalyzer_NoGrafanaClientReturnsWarning(t *testing.T) {
	res, err := New().Analyze(context.Background(), analyzers.Deps{
		Logger: slog.Default(),
		Limits: analyzers.DefaultLimits(),
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(res.Findings) != 0 {
		t.Errorf("Findings len = %d, want 0", len(res.Findings))
	}
	if len(res.Warnings) != 1 {
		t.Fatalf("Warnings len = %d, want 1; got %v", len(res.Warnings), res.Warnings)
	}
	if res.Warnings[0] != "grafana not configured: skipped" {
		t.Errorf("Warnings[0] = %q, want %q", res.Warnings[0], "grafana not configured: skipped")
	}
}
