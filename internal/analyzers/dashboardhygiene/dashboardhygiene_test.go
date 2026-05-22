// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package dashboardhygiene

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/remetric-dev/remetric/internal/analyzers"
	"github.com/remetric-dev/remetric/internal/findings"
	"github.com/remetric-dev/remetric/internal/grafana"
	"github.com/remetric-dev/remetric/internal/prometheus"
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

// stubsConfig defines the canned responses for the Prom + Grafana servers.
type stubsConfig struct {
	IngestedNames []string         // /api/v1/label/__name__/values
	Rules         []ruleStub       // /api/v1/rules
	Dashboards    []dashboardStub  // /api/search + /api/dashboards/uid/<uid>
	PromHandler   http.HandlerFunc // optional override for Prom
	GrafHandler   http.HandlerFunc // optional override for Grafana
}

type ruleStub struct {
	Name  string // recording rule output name
	Query string
	Type  string // "alerting" or "recording"
}

type dashboardStub struct {
	UID   string
	Title string
	URL   string // relative path Grafana would return in /api/search
	Body  string // raw JSON body for /api/dashboards/uid/<uid>
}

func newStubs(t *testing.T, cfg stubsConfig) (promURL, grafURL string) {
	t.Helper()
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cfg.PromHandler != nil {
			cfg.PromHandler(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/label/__name__/values":
			names := cfg.IngestedNames
			if names == nil {
				names = []string{}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": names})
		case "/api/v1/rules":
			rules := make([]map[string]any, 0, len(cfg.Rules))
			for _, rl := range cfg.Rules {
				rules = append(rules, map[string]any{"name": rl.Name, "query": rl.Query, "type": rl.Type})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"groups": []map[string]any{{"name": "g", "file": "f", "rules": rules}},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(prom.Close)

	graf := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if cfg.GrafHandler != nil {
			cfg.GrafHandler(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/api/search" {
			refs := make([]map[string]any, 0, len(cfg.Dashboards))
			for _, d := range cfg.Dashboards {
				refs = append(refs, map[string]any{"uid": d.UID, "title": d.Title, "url": d.URL})
			}
			_ = json.NewEncoder(w).Encode(refs)
			return
		}
		for _, d := range cfg.Dashboards {
			if r.URL.Path == "/api/dashboards/uid/"+d.UID {
				_, _ = w.Write([]byte(d.Body))
				return
			}
		}
		http.NotFound(w, r)
	}))
	t.Cleanup(graf.Close)
	return prom.URL, graf.URL
}

func mustClients(t *testing.T, promURL, grafURL string) (*prometheus.Client, *grafana.Client) {
	t.Helper()
	pc, err := prometheus.New(promURL, prometheus.WithFlavorHint(prometheus.FlavorProm))
	if err != nil {
		t.Fatalf("prometheus.New: %v", err)
	}
	gc, err := grafana.New(grafURL)
	if err != nil {
		t.Fatalf("grafana.New: %v", err)
	}
	return pc, gc
}

func TestAnalyzer_FlagsMissingMetric(t *testing.T) {
	promURL, grafURL := newStubs(t, stubsConfig{
		IngestedNames: []string{"up", "node_cpu_seconds_total"},
		Dashboards: []dashboardStub{{
			UID:   "d1",
			Title: "Frontend SLOs",
			URL:   "/d/d1/frontend-slos",
			Body: `{"dashboard":{"uid":"d1","title":"Frontend SLOs","panels":[
				{"type":"graph","title":"Latency p99","targets":[{"expr":"missing_metric_xyz","datasource":{"type":"prometheus"}}]}
			]}}`,
		}},
	})
	pc, gc := mustClients(t, promURL, grafURL)

	res, err := New().Analyze(context.Background(), analyzers.Deps{
		Prom:   pc,
		Graf:   gc,
		Logger: slog.Default(),
		Limits: analyzers.DefaultLimits(),
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("Findings len = %d, want 1; got %+v", len(res.Findings), res.Findings)
	}
	f := res.Findings[0]
	if f.Class != findings.ClassBrokenPanel {
		t.Errorf("Class = %q, want %q", f.Class, findings.ClassBrokenPanel)
	}
	if f.Category != findings.CategoryDashboardHygiene {
		t.Errorf("Category = %q, want %q", f.Category, findings.CategoryDashboardHygiene)
	}
	if f.Severity != findings.SeverityMedium {
		t.Errorf("Severity = %v, want Medium", f.Severity)
	}
	if f.Metric != "missing_metric_xyz" {
		t.Errorf("Metric = %q, want %q", f.Metric, "missing_metric_xyz")
	}
	if f.Dashboard != "Frontend SLOs" {
		t.Errorf("Dashboard = %q, want %q", f.Dashboard, "Frontend SLOs")
	}
}

func TestAnalyzer_ExistingMetricIsClean(t *testing.T) {
	promURL, grafURL := newStubs(t, stubsConfig{
		IngestedNames: []string{"up"},
		Dashboards: []dashboardStub{{
			UID:   "d1",
			Title: "Frontend SLOs",
			Body: `{"dashboard":{"uid":"d1","title":"Frontend SLOs","panels":[
				{"type":"graph","title":"P1","targets":[{"expr":"up","datasource":{"type":"prometheus"}}]}
			]}}`,
		}},
	})
	pc, gc := mustClients(t, promURL, grafURL)

	res, err := New().Analyze(context.Background(), analyzers.Deps{
		Prom: pc, Graf: gc, Logger: slog.Default(), Limits: analyzers.DefaultLimits(),
	})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(res.Findings) != 0 {
		t.Errorf("Findings len = %d, want 0; got %+v", len(res.Findings), res.Findings)
	}
}
