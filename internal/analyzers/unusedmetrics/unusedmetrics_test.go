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

	pc, _ := prometheus.New(prom.URL, prometheus.WithFlavorHint(prometheus.FlavorProm))
	gc, _ := grafana.New(graf.URL)

	res, err := New().Analyze(context.Background(), analyzers.Deps{
		Prom:   pc,
		Graf:   gc,
		Logger: slog.Default(),
		Limits: analyzers.DefaultLimits(),
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	out := res.Findings
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

	pc, _ := prometheus.New(prom.URL, prometheus.WithFlavorHint(prometheus.FlavorProm))
	res, err := New().Analyze(context.Background(), analyzers.Deps{
		Prom:   pc,
		Graf:   nil, // explicit
		Logger: slog.Default(),
		Limits: analyzers.DefaultLimits(),
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	out := res.Findings
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

	pc, _ := prometheus.New(prom.URL, prometheus.WithFlavorHint(prometheus.FlavorProm))
	res, err := New().Analyze(context.Background(), analyzers.Deps{
		Prom: pc, Graf: nil, Logger: slog.Default(), Limits: analyzers.DefaultLimits(),
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	out := res.Findings
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

func TestAnalyzer_VictoriaWithoutVMAlert_WarnsAndContinues(t *testing.T) {
	// Prom-side serves labels + tsdb + an HTTP 200 empty-groups rules
	// payload (the actual VictoriaMetrics single-node behavior). Flavor
	// is forced Victoria. The analyzer must still surface the
	// "rules unavailable" warning because without --vmalert we can't
	// distinguish "no rules" from "rules live in vmalert".
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/label/__name__/values":
			_, _ = w.Write([]byte(`{"status":"success","data":["only_metric"]}`))
		case "/api/v1/status/tsdb":
			_, _ = w.Write([]byte(`{"status":"success","data":{"seriesCountByMetricName":[{"name":"only_metric","value":100}]}}`))
		case "/api/v1/rules":
			_, _ = w.Write([]byte(`{"status":"success","data":{"groups":[]}}`))
		default:
			http.Error(w, "unexpected: "+r.URL.Path, 500)
		}
	}))
	defer srv.Close()
	prom, err := prometheus.New(srv.URL, prometheus.WithFlavorHint(prometheus.FlavorVictoria))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	a := New()
	res, err := a.Analyze(context.Background(), analyzers.Deps{Prom: prom})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(res.Warnings) == 0 {
		t.Fatalf("expected at least one warning, got none")
	}
	wantSub := "rules unavailable"
	if !strings.Contains(res.Warnings[0], wantSub) {
		t.Errorf("warning %q does not contain %q", res.Warnings[0], wantSub)
	}
	if len(res.Findings) != 1 {
		t.Errorf("Findings len = %d, want 1 (analyzer should continue with empty rules)", len(res.Findings))
	}
}

func TestAnalyzer_VictoriaWithVMAlert_UsesVMAlertForRules(t *testing.T) {
	promHits := map[string]int{}
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		promHits[r.URL.Path]++
		switch r.URL.Path {
		case "/api/v1/label/__name__/values":
			_, _ = w.Write([]byte(`{"status":"success","data":["only_metric","record_metric"]}`))
		case "/api/v1/status/tsdb":
			_, _ = w.Write([]byte(`{"status":"success","data":{"seriesCountByMetricName":[{"name":"only_metric","value":100}]}}`))
		case "/api/v1/rules":
			w.WriteHeader(404) // VM never serves rules itself
		default:
			http.Error(w, "unexpected: "+r.URL.Path, 500)
		}
	}))
	defer prom.Close()
	vmalert := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/rules" {
			_, _ = w.Write([]byte(`{"status":"success","data":{"groups":[{"name":"g","rules":[{"name":"record_metric","query":"sum(only_metric)","type":"recording"}]}]}}`))
			return
		}
		http.Error(w, "wrong path", 500)
	}))
	defer vmalert.Close()
	pClient, _ := prometheus.New(prom.URL, prometheus.WithFlavorHint(prometheus.FlavorVictoria))
	vClient, _ := prometheus.New(vmalert.URL, prometheus.WithFlavorHint(prometheus.FlavorProm))
	a := New()
	res, err := a.Analyze(context.Background(), analyzers.Deps{Prom: pClient, VMAlert: vClient})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(res.Warnings) != 0 {
		t.Errorf("expected no warnings, got %v", res.Warnings)
	}
	// only_metric is referenced by the recording rule's query, record_metric is the output.
	// Both should be "used" → zero findings.
	if len(res.Findings) != 0 {
		t.Errorf("Findings = %+v, want empty", res.Findings)
	}
	if promHits["/api/v1/rules"] != 0 {
		t.Errorf("prom rules endpoint should not be hit when VMAlert is set; hits=%d", promHits["/api/v1/rules"])
	}
}
