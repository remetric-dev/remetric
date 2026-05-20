package cardinality_test

import (
	"context"
	"strings"
	"testing"

	"github.com/remetric-dev/remetric/internal/analyzers"
	"github.com/remetric-dev/remetric/internal/analyzers/cardinality"
	"github.com/remetric-dev/remetric/internal/findings"
	prom "github.com/remetric-dev/remetric/internal/prometheus"
	"github.com/remetric-dev/remetric/internal/prometheus/promtest"
)

func newDeps(t *testing.T, baseURL string) analyzers.Deps {
	t.Helper()
	c, err := prom.New(baseURL)
	if err != nil {
		t.Fatalf("prom.New: %v", err)
	}
	return analyzers.Deps{Prom: c, Limits: analyzers.DefaultLimits()}
}

func TestCardinalityAnalyzer_TopMetric(t *testing.T) {
	srv := promtest.NewServer(t, "testdata", promtest.Routes{
		"/api/v1/status/tsdb":                        "tsdb_one_bomb.json",
		"/api/v1/labels":                             "labels_one_bomb.json",
		"/api/v1/label/destination_principal/values": "label_values_dp.json",
		"/api/v1/label/response_code/values":         "label_values_rc.json",
	})

	a := cardinality.New()
	got, err := a.Analyze(context.Background(), newDeps(t, srv.URL))
	if err != nil {
		t.Fatalf("Analyze err = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("len(findings) = %d, want 1", len(got))
	}
	f := got[0]
	if f.Severity != findings.SeverityCritical {
		t.Errorf("Severity = %v, want Critical", f.Severity)
	}
	if f.Metric != "istio_requests_total" {
		t.Errorf("Metric = %q, want istio_requests_total", f.Metric)
	}
	if f.Evidence.Label != "destination_principal" {
		t.Errorf("Evidence.Label = %q, want destination_principal", f.Evidence.Label)
	}
	if f.Evidence.UniqueValues != 124 {
		t.Errorf("UniqueValues = %d, want 124", f.Evidence.UniqueValues)
	}
	if !strings.Contains(f.Fix.Config, "labeldrop") {
		t.Errorf("Fix.Config missing labeldrop:\n%s", f.Fix.Config)
	}
	wantID := "card-istio_requests_total-destination_principal"
	if f.ID != wantID {
		t.Errorf("ID = %q, want %q", f.ID, wantID)
	}
}

func TestCardinalityAnalyzer_EmptyTSDB(t *testing.T) {
	srv := promtest.NewServer(t, "testdata", promtest.Routes{
		"/api/v1/status/tsdb": "tsdb_empty.json",
	})
	a := cardinality.New()
	got, err := a.Analyze(context.Background(), newDeps(t, srv.URL))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 0 {
		t.Errorf("findings = %d, want 0", len(got))
	}
}

func TestCardinalityAnalyzer_DeterministicIDs(t *testing.T) {
	srv := promtest.NewServer(t, "testdata", promtest.Routes{
		"/api/v1/status/tsdb":                        "tsdb_one_bomb.json",
		"/api/v1/labels":                             "labels_one_bomb.json",
		"/api/v1/label/destination_principal/values": "label_values_dp.json",
		"/api/v1/label/response_code/values":         "label_values_rc.json",
	})
	a := cardinality.New()
	a1, err := a.Analyze(context.Background(), newDeps(t, srv.URL))
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	a2, err := a.Analyze(context.Background(), newDeps(t, srv.URL))
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if a1[0].ID != a2[0].ID {
		t.Errorf("ID drift: %q vs %q", a1[0].ID, a2[0].ID)
	}
}

func TestCardinalityAnalyzer_ImpactUpperBound(t *testing.T) {
	srv := promtest.NewServer(t, "testdata", promtest.Routes{
		"/api/v1/status/tsdb":                        "tsdb_one_bomb.json",
		"/api/v1/labels":                             "labels_one_bomb.json",
		"/api/v1/label/destination_principal/values": "label_values_dp.json",
		"/api/v1/label/response_code/values":         "label_values_rc.json",
	})
	a := cardinality.New()
	got, err := a.Analyze(context.Background(), newDeps(t, srv.URL))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	// 500_000 - 500_000/124 = 500000 - 4032 = 495968
	if got[0].Impact.SeriesReduction != 495968 {
		t.Errorf("SeriesReduction = %d, want 495968", got[0].Impact.SeriesReduction)
	}
	if got[0].Impact.EstimationMethod != "labeldrop_upper_bound" {
		t.Errorf("EstimationMethod = %q, want labeldrop_upper_bound", got[0].Impact.EstimationMethod)
	}
}

func TestCardinalityAnalyzer_SampleSizeBounded(t *testing.T) {
	srv := promtest.NewServer(t, "testdata", promtest.Routes{
		"/api/v1/status/tsdb":                        "tsdb_one_bomb.json",
		"/api/v1/labels":                             "labels_one_bomb.json",
		"/api/v1/label/destination_principal/values": "label_values_dp.json",
		"/api/v1/label/response_code/values":         "label_values_rc.json",
	})
	a := cardinality.New()
	got, err := a.Analyze(context.Background(), newDeps(t, srv.URL))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if n := len(got[0].Evidence.SampleValues); n != 5 {
		t.Errorf("SampleValues len = %d, want 5", n)
	}
}
