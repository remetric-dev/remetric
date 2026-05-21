// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package labelpattern

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/remetric-dev/remetric/internal/analyzers"
	"github.com/remetric-dev/remetric/internal/findings"
	"github.com/remetric-dev/remetric/internal/prometheus"
)

func TestAnalyzer_Name(t *testing.T) {
	if got := New().Name(); got != "labelpattern" {
		t.Errorf("Name() = %q, want %q", got, "labelpattern")
	}
}

func TestAnalyzer_DetectsIdSuffix(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/status/tsdb"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"headStats": map[string]any{
						"numSeries": 1_000_000,
					},
					"seriesCountByMetricName": []map[string]any{
						{"name": "http_requests_total", "value": 800_000},
					},
					"labelValueCountByLabelName": []map[string]any{
						{"name": "user_id", "value": 5_000},
						{"name": "namespace", "value": 200},
						{"name": "status", "value": 5},
					},
				},
			})
		case r.URL.Path == "/api/v1/label/__name__/values":
			if got := r.URL.Query().Get("match[]"); got == `{user_id!=""}` {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"status": "success",
					"data":   []string{"http_requests_total"},
				})
				return
			}
			http.Error(w, "unexpected match[]: "+r.URL.Query().Get("match[]"), http.StatusBadRequest)
		case r.URL.Path == "/api/v1/label/user_id/values":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   []string{"u-1", "u-2", "u-3", "u-4", "u-5", "u-6"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	c, err := prometheus.New(ts.URL)
	if err != nil {
		t.Fatalf("prometheus.New: %v", err)
	}

	d := analyzers.Deps{
		Prom:   c,
		Logger: slog.Default(),
		Limits: analyzers.DefaultLimits(),
	}

	res, err := New().Analyze(context.Background(), d)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if len(res.Findings) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(res.Findings), res.Findings)
	}
	f := res.Findings[0]
	if f.Category != findings.CategoryLabelPatterns {
		t.Errorf("Category = %v, want %v", f.Category, findings.CategoryLabelPatterns)
	}
	if f.Severity != findings.SeverityHigh {
		// 5000 unique values → High (LabelPatternSeverity: >1000 → High, >5000 → Critical, 5000 not > 5000)
		t.Errorf("Severity = %v, want High", f.Severity)
	}
	if f.Evidence.Label != "user_id" {
		t.Errorf("Label = %q, want %q", f.Evidence.Label, "user_id")
	}
	if f.Evidence.UniqueValues != 5000 {
		t.Errorf("UniqueValues = %d, want 5000", f.Evidence.UniqueValues)
	}
	if f.Fix.Type != "drop_label" {
		t.Errorf("Fix.Type = %q, want drop_label", f.Fix.Type)
	}
	if !strings.Contains(f.Fix.Config, "user_id") {
		t.Errorf("Fix.Config missing label name:\n%s", f.Fix.Config)
	}
}

func TestAnalyzer_IgnoresBoundedLabels(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if strings.HasPrefix(r.URL.Path, "/api/v1/status/tsdb") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"headStats": map[string]any{"numSeries": 1_000_000},
					"seriesCountByMetricName": []map[string]any{
						{"name": "http_requests_total", "value": 800_000},
					},
					"labelValueCountByLabelName": []map[string]any{
						{"name": "namespace", "value": 10_000},
						{"name": "instance", "value": 2_000},
					},
				},
			})
			return
		}
		http.NotFound(w, r)
	}))
	defer ts.Close()

	c, _ := prometheus.New(ts.URL)
	res, err := New().Analyze(context.Background(), analyzers.Deps{
		Prom:   c,
		Logger: slog.Default(),
		Limits: analyzers.DefaultLimits(),
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(res.Findings) != 0 {
		t.Errorf("expected 0 findings (bounded labels ignored), got %d: %+v", len(res.Findings), res.Findings)
	}
}

func TestAnalyzer_SortsBySeverityThenUnique(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/status/tsdb"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"headStats":               map[string]any{"numSeries": 10_000_000},
					"seriesCountByMetricName": []map[string]any{{"name": "m", "value": 1000}},
					"labelValueCountByLabelName": []map[string]any{
						{"name": "user_id", "value": 600},   // medium (>100, <=1000)
						{"name": "trace_id", "value": 6000}, // critical (>5000)
						{"name": "span_id", "value": 1500},  // high (>1000, <=5000)
					},
				},
			})
		case r.URL.Path == "/api/v1/label/__name__/values":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   []string{"m"},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   []string{"x"},
			})
		}
	}))
	defer ts.Close()

	c, _ := prometheus.New(ts.URL)
	res, err := New().Analyze(context.Background(), analyzers.Deps{
		Prom:   c,
		Logger: slog.Default(),
		Limits: analyzers.DefaultLimits(),
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	wantOrder := []string{"trace_id", "span_id", "user_id"}
	gotOrder := make([]string, 0, len(res.Findings))
	for _, f := range res.Findings {
		gotOrder = append(gotOrder, f.Evidence.Label)
	}
	if diff := cmp.Diff(wantOrder, gotOrder, cmpopts.EquateEmpty()); diff != "" {
		t.Errorf("sort order mismatch (-want +got):\n%s", diff)
	}
}

func TestNewWithPatterns_Happy(t *testing.T) {
	a, err := NewWithPatterns([]string{`^foo$`})
	if err != nil {
		t.Fatalf("NewWithPatterns: %v", err)
	}
	if a == nil {
		t.Fatal("nil analyzer")
	}
	if a.Name() != "labelpattern" {
		t.Errorf("Name = %q, want labelpattern", a.Name())
	}
}

func TestNewWithPatterns_InvalidRegex(t *testing.T) {
	_, err := NewWithPatterns([]string{`[unclosed`})
	if err == nil {
		t.Errorf("expected error for invalid regex, got nil")
	}
}

func TestAnalyzer_HeuristicDowngradesBoundedValues(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/status/tsdb"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"headStats":               map[string]any{"numSeries": 1_000_000},
					"seriesCountByMetricName": []map[string]any{{"name": "m", "value": 100}},
					"labelValueCountByLabelName": []map[string]any{
						{"name": "user_id", "value": 5_000}, // would be High
					},
				},
			})
		case r.URL.Path == "/api/v1/label/__name__/values":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": []string{"m"}})
		case r.URL.Path == "/api/v1/label/user_id/values":
			// Bounded-looking samples: short digits.
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": []string{"1", "2", "3", "4", "5"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	c, _ := prometheus.New(ts.URL)
	res, err := New().Analyze(context.Background(), analyzers.Deps{
		Prom: c, Logger: slog.Default(), Limits: analyzers.DefaultLimits(),
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("Findings = %d, want 1", len(res.Findings))
	}
	if res.Findings[0].Severity != findings.SeverityMedium {
		t.Errorf("Severity = %v, want Medium (downgraded from High)", res.Findings[0].Severity)
	}
	if !strings.Contains(res.Findings[0].Evidence.Description, "look bounded") {
		t.Errorf("Description = %q, want it to mention 'look bounded'", res.Findings[0].Evidence.Description)
	}
}

func TestAnalyzer_HeuristicKeepsUnboundedSeverity(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/status/tsdb"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"headStats":               map[string]any{"numSeries": 1_000_000},
					"seriesCountByMetricName": []map[string]any{{"name": "m", "value": 100}},
					"labelValueCountByLabelName": []map[string]any{
						{"name": "user_id", "value": 5_000},
					},
				},
			})
		case r.URL.Path == "/api/v1/label/__name__/values":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": []string{"m"}})
		case r.URL.Path == "/api/v1/label/user_id/values":
			// UUID-like samples → unbounded.
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": []string{"u-abc-1", "u-xyz-99", "u-qqq-42"}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()
	c, _ := prometheus.New(ts.URL)
	res, err := New().Analyze(context.Background(), analyzers.Deps{
		Prom: c, Logger: slog.Default(), Limits: analyzers.DefaultLimits(),
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("Findings = %d, want 1", len(res.Findings))
	}
	if res.Findings[0].Severity != findings.SeverityHigh {
		t.Errorf("Severity = %v, want High (NOT downgraded)", res.Findings[0].Severity)
	}
	if strings.Contains(res.Findings[0].Evidence.Description, "look bounded") {
		t.Errorf("Description = %q, must NOT mention 'look bounded'", res.Findings[0].Evidence.Description)
	}
}
