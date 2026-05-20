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

	got, err := New().Analyze(context.Background(), d)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1: %+v", len(got), got)
	}
	f := got[0]
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
	got, err := New().Analyze(context.Background(), analyzers.Deps{
		Prom:   c,
		Logger: slog.Default(),
		Limits: analyzers.DefaultLimits(),
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected 0 findings (bounded labels ignored), got %d: %+v", len(got), got)
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
	got, err := New().Analyze(context.Background(), analyzers.Deps{
		Prom:   c,
		Logger: slog.Default(),
		Limits: analyzers.DefaultLimits(),
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	wantOrder := []string{"trace_id", "span_id", "user_id"}
	gotOrder := make([]string, 0, len(got))
	for _, f := range got {
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
