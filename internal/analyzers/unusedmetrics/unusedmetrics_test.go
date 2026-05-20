// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package unusedmetrics

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/remetric-dev/remetric/internal/analyzers"
	"github.com/remetric-dev/remetric/internal/findings"
	"github.com/remetric-dev/remetric/internal/grafana"
	"github.com/remetric-dev/remetric/internal/prometheus"
)

func TestAnalyzer_Name(t *testing.T) {
	if got := New().Name(); got != "unusedmetrics" {
		t.Errorf("Name = %q, want %q", got, "unusedmetrics")
	}
}

func TestAnalyzer_DiffsCorrectly(t *testing.T) {
	// Prom mock: 3 ingested metrics (a, b, c), TSDB stats for series counts.
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/label/__name__/values":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   []string{"a", "b", "c"},
			})
		case strings.HasPrefix(r.URL.Path, "/api/v1/status/tsdb"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"headStats": map[string]any{"numSeries": 1000},
					"seriesCountByMetricName": []map[string]any{
						{"name": "a", "value": 10_000},
						{"name": "b", "value": 6_000},
						{"name": "c", "value": 100_000_000},
					},
				},
			})
		case r.URL.Path == "/api/v1/rules":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"groups": []map[string]any{
						{
							"name":  "g",
							"file":  "f",
							"rules": []map[string]any{{"name": "AlertA", "query": "a > 0", "type": "alerting"}},
						},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer prom.Close()

	// Grafana mock: 1 dashboard using b
	graf := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/search":
			_ = json.NewEncoder(w).Encode([]map[string]any{{"uid": "d1", "title": "T"}})
		case "/api/dashboards/uid/d1":
			_, _ = w.Write([]byte(`{"dashboard":{"uid":"d1","title":"T","panels":[
				{"type":"graph","targets":[{"expr":"rate(b[5m])","datasource":{"type":"prometheus"}}]}
			]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer graf.Close()

	pc, _ := prometheus.New(prom.URL)
	gc, _ := grafana.New(graf.URL)

	out, err := New().Analyze(context.Background(), analyzers.Deps{
		Prom:   pc,
		Graf:   gc,
		Logger: slog.Default(),
		Limits: analyzers.DefaultLimits(),
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	gotNames := make([]string, 0, len(out))
	for _, f := range out {
		gotNames = append(gotNames, f.Metric)
	}
	sort.Strings(gotNames)
	// `a` used by alert rule, `b` used by dashboard → unused = {c}
	want := []string{"c"}
	if diff := cmp.Diff(want, gotNames); diff != "" {
		t.Errorf("unused metrics mismatch (-want +got):\n%s", diff)
	}
	if out[0].Severity != findings.SeverityCritical {
		t.Errorf("severity for c (100M series) = %v, want Critical", out[0].Severity)
	}
	if out[0].Category != findings.CategoryUnusedMetrics {
		t.Errorf("category = %v, want CategoryUnusedMetrics", out[0].Category)
	}
	if out[0].Fix.Type != "drop_metric" {
		t.Errorf("fix.type = %q, want drop_metric", out[0].Fix.Type)
	}
}

func TestAnalyzer_GrafanaNilSkipsDashboards(t *testing.T) {
	// Prom mock: ingested = {a}, no rules
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/label/__name__/values":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": []string{"a"}})
		case strings.HasPrefix(r.URL.Path, "/api/v1/status/tsdb"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"headStats":               map[string]any{"numSeries": 10},
					"seriesCountByMetricName": []map[string]any{{"name": "a", "value": 10}},
				},
			})
		case r.URL.Path == "/api/v1/rules":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   map[string]any{"groups": []map[string]any{}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer prom.Close()

	pc, _ := prometheus.New(prom.URL)
	out, err := New().Analyze(context.Background(), analyzers.Deps{
		Prom:   pc,
		Graf:   nil, // explicit
		Logger: slog.Default(),
		Limits: analyzers.DefaultLimits(),
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(out) != 1 || out[0].Metric != "a" {
		t.Errorf("expected 1 finding for `a`, got %+v", out)
	}
}

func TestAnalyzer_RecordingRuleOutputUsed(t *testing.T) {
	// recording rule output `foo:5m` from query `rate(other[5m])`
	// Ingested: {foo:5m, other}
	// Expect zero unused (foo:5m is recording output → used; other is recording input → used).
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/label/__name__/values":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": []string{"foo:5m", "other"}})
		case strings.HasPrefix(r.URL.Path, "/api/v1/status/tsdb"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"headStats": map[string]any{"numSeries": 10},
					"seriesCountByMetricName": []map[string]any{
						{"name": "foo:5m", "value": 5},
						{"name": "other", "value": 5},
					},
				},
			})
		case r.URL.Path == "/api/v1/rules":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"groups": []map[string]any{
						{"name": "g", "file": "f", "rules": []map[string]any{
							{"name": "foo:5m", "query": "rate(other[5m])", "type": "recording"},
						}},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer prom.Close()

	pc, _ := prometheus.New(prom.URL)
	out, err := New().Analyze(context.Background(), analyzers.Deps{
		Prom: pc, Graf: nil, Logger: slog.Default(), Limits: analyzers.DefaultLimits(),
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(out) != 0 {
		gotNames := make([]string, 0, len(out))
		for _, f := range out {
			gotNames = append(gotNames, f.Metric)
		}
		t.Errorf("expected no unused metrics, got %v", gotNames)
	}
}

func TestAnalyzer_PrometheusError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer ts.Close()
	pc, _ := prometheus.New(ts.URL)
	_, err := New().Analyze(context.Background(), analyzers.Deps{
		Prom: pc, Logger: slog.Default(), Limits: analyzers.DefaultLimits(),
	})
	if err == nil {
		t.Error("expected error from prom 500, got nil")
	}
}

func TestAnalyzer_GrafanaError(t *testing.T) {
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/label/__name__/values":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": []string{"a"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer prom.Close()
	graf := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer graf.Close()
	pc, _ := prometheus.New(prom.URL)
	gc, _ := grafana.New(graf.URL)
	_, err := New().Analyze(context.Background(), analyzers.Deps{
		Prom: pc, Graf: gc, Logger: slog.Default(), Limits: analyzers.DefaultLimits(),
	})
	if err == nil {
		t.Error("expected error from grafana 500, got nil")
	}
}
