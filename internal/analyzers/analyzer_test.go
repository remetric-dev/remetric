package analyzers_test

import (
	"context"
	"testing"

	"github.com/remetric-dev/remetric/internal/analyzers"
	"github.com/remetric-dev/remetric/internal/findings"
)

type fakeAnalyzer struct{}

func (fakeAnalyzer) Name() string { return "fake" }
func (fakeAnalyzer) Analyze(_ context.Context, _ analyzers.Deps) ([]findings.Finding, error) {
	return []findings.Finding{{ID: "x", Severity: findings.SeverityHigh}}, nil
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
	if len(out) != 1 || out[0].ID != "x" {
		t.Errorf("out = %+v, want one finding id=x", out)
	}
}

func TestDefaultLimits(t *testing.T) {
	l := analyzers.DefaultLimits()
	if l.TopMetrics != 20 {
		t.Errorf("TopMetrics = %d, want 20", l.TopMetrics)
	}
	if l.SampleSize != 5 {
		t.Errorf("SampleSize = %d, want 5", l.SampleSize)
	}
}
