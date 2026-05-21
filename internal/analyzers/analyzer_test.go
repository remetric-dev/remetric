// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package analyzers_test

import (
	"context"
	"testing"

	"github.com/remetric-dev/remetric/internal/analyzers"
	"github.com/remetric-dev/remetric/internal/findings"
)

func TestDefaultLimits(t *testing.T) {
	l := analyzers.DefaultLimits()
	if l.TopMetrics != 20 {
		t.Errorf("TopMetrics = %d, want 20", l.TopMetrics)
	}
	if l.SampleSize != 5 {
		t.Errorf("SampleSize = %d, want 5", l.SampleSize)
	}
}

func TestResult_ZeroValueHasNoWarningsOrFindings(t *testing.T) {
	var r analyzers.Result
	if len(r.Findings) != 0 || len(r.Warnings) != 0 {
		t.Errorf("zero Result not empty: %+v", r)
	}
}

type fakeAnalyzer struct{}

func (fakeAnalyzer) Name() string { return "fake" }
func (fakeAnalyzer) Analyze(_ context.Context, _ analyzers.Deps) (analyzers.Result, error) {
	return analyzers.Result{
		Findings: []findings.Finding{{ID: "fake-1"}},
		Warnings: []string{"warning one"},
	}, nil
}

func TestAnalyzer_InterfaceShape(t *testing.T) {
	var a analyzers.Analyzer = fakeAnalyzer{}
	if a.Name() != "fake" {
		t.Errorf("Name = %q, want fake", a.Name())
	}
	out, err := a.Analyze(context.Background(), analyzers.Deps{})
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(out.Findings) != 1 || out.Findings[0].ID != "fake-1" {
		t.Errorf("Findings = %+v, want one finding id=fake-1", out.Findings)
	}
	if len(out.Warnings) != 1 || out.Warnings[0] != "warning one" {
		t.Errorf("Warnings = %+v, want one warning", out.Warnings)
	}
}
