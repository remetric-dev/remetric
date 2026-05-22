// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package cardinality_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

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
	res, err := a.Analyze(context.Background(), newDeps(t, srv.URL))
	if err != nil {
		t.Fatalf("Analyze err = %v", err)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("len(findings) = %d, want 1", len(res.Findings))
	}
	f := res.Findings[0]
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
	if f.Category != findings.CategoryCardinality {
		t.Errorf("Category = %q, want %q", f.Category, findings.CategoryCardinality)
	}
}

func TestCardinalityAnalyzer_Name(t *testing.T) {
	if got, want := cardinality.New().Name(), "cardinality"; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}

func TestCardinalityAnalyzer_SortsBySeverity(t *testing.T) {
	srv := promtest.NewServer(t, "testdata", promtest.Routes{
		"/api/v1/status/tsdb":       "tsdb_two_bombs.json",
		"/api/v1/labels":            "labels_big.json",
		"/api/v1/label/kind/values": "label_values_big_kind.json",
	})
	a := cardinality.New()
	res, err := a.Analyze(context.Background(), newDeps(t, srv.URL))
	if err != nil {
		t.Fatalf("Analyze err = %v", err)
	}
	if len(res.Findings) != 2 {
		t.Fatalf("len(findings) = %d, want 2", len(res.Findings))
	}
	if res.Findings[0].Severity != findings.SeverityCritical {
		t.Errorf("first.Severity = %v, want Critical", res.Findings[0].Severity)
	}
	if res.Findings[1].Severity != findings.SeverityHigh {
		t.Errorf("second.Severity = %v, want High", res.Findings[1].Severity)
	}
	if res.Findings[0].Metric != "big_metric_second" {
		t.Errorf("first.Metric = %q, want big_metric_second", res.Findings[0].Metric)
	}
}

func TestCardinalityAnalyzer_PrometheusError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	a := cardinality.New()
	_, err := a.Analyze(context.Background(), newDeps(t, srv.URL))
	if err == nil {
		t.Errorf("Analyze err = nil, want propagated 500")
	}
}

func TestCardinalityAnalyzer_EmptyTSDB(t *testing.T) {
	srv := promtest.NewServer(t, "testdata", promtest.Routes{
		"/api/v1/status/tsdb": "tsdb_empty.json",
	})
	a := cardinality.New()
	res, err := a.Analyze(context.Background(), newDeps(t, srv.URL))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(res.Findings) != 0 {
		t.Errorf("findings = %d, want 0", len(res.Findings))
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
	r1, err := a.Analyze(context.Background(), newDeps(t, srv.URL))
	if err != nil {
		t.Fatalf("first run: %v", err)
	}
	r2, err := a.Analyze(context.Background(), newDeps(t, srv.URL))
	if err != nil {
		t.Fatalf("second run: %v", err)
	}
	if r1.Findings[0].ID != r2.Findings[0].ID {
		t.Errorf("ID drift: %q vs %q", r1.Findings[0].ID, r2.Findings[0].ID)
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
	res, err := a.Analyze(context.Background(), newDeps(t, srv.URL))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	// 500_000 - 500_000/124 = 500000 - 4032 = 495968
	if res.Findings[0].Impact.SeriesReduction != 495968 {
		t.Errorf("SeriesReduction = %d, want 495968", res.Findings[0].Impact.SeriesReduction)
	}
	if res.Findings[0].Impact.EstimationMethod != "labeldrop_upper_bound" {
		t.Errorf("EstimationMethod = %q, want labeldrop_upper_bound", res.Findings[0].Impact.EstimationMethod)
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
	res, err := a.Analyze(context.Background(), newDeps(t, srv.URL))
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if n := len(res.Findings[0].Evidence.SampleValues); n != 5 {
		t.Errorf("SampleValues len = %d, want 5", n)
	}
}

func TestCardinalityAnalyzer_PerMetricErrorAggregation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/status/tsdb"):
			_, _ = w.Write([]byte(`{"status":"success","data":{
				"headStats":{"numSeries":1000},
				"seriesCountByMetricName":[
					{"name":"good_metric","value":800},
					{"name":"broken_metric","value":700}
				]
			}}`))
		case r.URL.Path == "/api/v1/labels":
			// Per-metric labels endpoint: ?match[]={__name__="..."}
			match := r.URL.Query().Get("match[]")
			if strings.Contains(match, `"good_metric"`) {
				_, _ = w.Write([]byte(`{"status":"success","data":["lbl_a"]}`))
				return
			}
			if strings.Contains(match, `"broken_metric"`) {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			http.Error(w, "unexpected match[]: "+match, 400)
		case r.URL.Path == "/api/v1/label/lbl_a/values":
			_, _ = w.Write([]byte(`{"status":"success","data":["v1","v2","v3","v4","v5"]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	a := cardinality.New()
	res, err := a.Analyze(context.Background(), newDeps(t, srv.URL))
	if err != nil {
		t.Fatalf("Analyze returned error, want graceful: %v", err)
	}
	// good_metric should still produce a finding (or be filtered by severity);
	// the key assertion is no hard error AND a warning for broken_metric.
	if len(res.Warnings) != 1 {
		t.Fatalf("Warnings = %v, want exactly 1", res.Warnings)
	}
	if !strings.Contains(res.Warnings[0], `"broken_metric"`) {
		t.Errorf("Warnings[0] = %q, want it to mention broken_metric", res.Warnings[0])
	}
	if !strings.Contains(res.Warnings[0], "cardinality:") {
		t.Errorf("Warnings[0] = %q, want cardinality: prefix", res.Warnings[0])
	}
}

func TestCardinalityAnalyzer_WorstLabel_PicksBestAmongSuccesses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/status/tsdb"):
			_, _ = w.Write([]byte(`{"status":"success","data":{
				"headStats":{"numSeries":1000},
				"seriesCountByMetricName":[{"name":"m","value":500}]
			}}`))
		case r.URL.Path == "/api/v1/labels":
			_, _ = w.Write([]byte(`{"status":"success","data":["a","b","c"]}`))
		case r.URL.Path == "/api/v1/label/a/values":
			// 3 values
			_, _ = w.Write([]byte(`{"status":"success","data":["1","2","3"]}`))
		case r.URL.Path == "/api/v1/label/b/values":
			// HTTP 500 - this label "fails"
			w.WriteHeader(http.StatusInternalServerError)
		case r.URL.Path == "/api/v1/label/c/values":
			// 10 values - the largest successful
			_, _ = w.Write([]byte(`{"status":"success","data":["1","2","3","4","5","6","7","8","9","10"]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	a := cardinality.New()
	res, err := a.Analyze(context.Background(), newDeps(t, srv.URL))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("Findings = %d, want 1", len(res.Findings))
	}
	if got := res.Findings[0].Evidence.Label; got != "c" {
		t.Errorf("Evidence.Label = %q, want %q (largest of successes)", got, "c")
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], `"b"`) {
		t.Errorf("Warnings = %v, want 1 entry mentioning label b", res.Warnings)
	}
}

func TestCardinalityAnalyzer_WorstLabel_AllLabelsFailSkipsMetric(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/status/tsdb"):
			_, _ = w.Write([]byte(`{"status":"success","data":{
				"headStats":{"numSeries":1000},
				"seriesCountByMetricName":[{"name":"m","value":500}]
			}}`))
		case r.URL.Path == "/api/v1/labels":
			_, _ = w.Write([]byte(`{"status":"success","data":["x","y"]}`))
		case r.URL.Path == "/api/v1/label/x/values", r.URL.Path == "/api/v1/label/y/values":
			w.WriteHeader(http.StatusInternalServerError)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	a := cardinality.New()
	res, err := a.Analyze(context.Background(), newDeps(t, srv.URL))
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(res.Findings) != 0 {
		t.Errorf("Findings = %+v, want empty (no successful label)", res.Findings)
	}
	if len(res.Warnings) != 2 {
		t.Errorf("Warnings count = %d, want 2 (one per failed label)", len(res.Warnings))
	}
}

func TestCardinalityAnalyzer_WorstLabel_HonoursPromSemaphore(t *testing.T) {
	const maxInFlight = 2
	var (
		mu             sync.Mutex
		concurrent     int
		peakConcurrent int
	)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/status/tsdb"):
			_, _ = w.Write([]byte(`{"status":"success","data":{
				"headStats":{"numSeries":1000},
				"seriesCountByMetricName":[{"name":"m","value":500}]
			}}`))
		case r.URL.Path == "/api/v1/labels":
			_, _ = w.Write([]byte(`{"status":"success","data":["a","b","c","d","e","f","g","h"]}`))
		case strings.HasPrefix(r.URL.Path, "/api/v1/label/") && strings.HasSuffix(r.URL.Path, "/values"):
			mu.Lock()
			concurrent++
			if concurrent > peakConcurrent {
				peakConcurrent = concurrent
			}
			mu.Unlock()
			time.Sleep(50 * time.Millisecond) // hold the slot so contention is observable
			mu.Lock()
			concurrent--
			mu.Unlock()
			_, _ = w.Write([]byte(`{"status":"success","data":["v1","v2"]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	c, err := prom.New(srv.URL, prom.WithMaxInFlight(maxInFlight))
	if err != nil {
		t.Fatalf("prom.New: %v", err)
	}
	a := cardinality.New()
	_, err = a.Analyze(context.Background(), analyzers.Deps{Prom: c, Limits: analyzers.DefaultLimits()})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	mu.Lock()
	got := peakConcurrent
	mu.Unlock()
	if got > maxInFlight {
		t.Errorf("peak concurrent = %d, must be <= %d", got, maxInFlight)
	}
	if got < 2 {
		t.Errorf("peak concurrent = %d, want >= 2 (parallelism actually happened)", got)
	}
}
