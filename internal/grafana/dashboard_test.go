// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package grafana

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDashboard_FetchAndParse(t *testing.T) {
	body := `{
	  "dashboard": {
	    "uid": "abc",
	    "title": "Sample",
	    "panels": [
	      {
	        "type": "graph",
	        "targets": [
	          {"expr": "rate(http_requests_total[5m])", "datasource": {"type": "prometheus"}}
	        ]
	      }
	    ]
	  }
	}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/dashboards/uid/abc" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer ts.Close()

	c, _ := New(ts.URL)
	dash, err := c.Dashboard(context.Background(), "abc")
	if err != nil {
		t.Fatalf("Dashboard: %v", err)
	}
	if dash.UID != "abc" || dash.Title != "Sample" {
		t.Errorf("got %+v, want UID=abc Title=Sample", dash)
	}

	got := dash.Queries()
	want := []string{"rate(http_requests_total[5m])"}
	sort.Strings(got)
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Queries mismatch (-want +got):\n%s", diff)
	}
}

func TestDashboard_Queries_NestedPanels(t *testing.T) {
	body := `{
	  "dashboard": {
	    "uid": "x",
	    "title": "Nested",
	    "panels": [
	      {
	        "type": "row",
	        "panels": [
	          {"type": "graph", "targets": [{"expr": "up", "datasource": {"type": "prometheus"}}]}
	        ]
	      },
	      {"type": "stat", "targets": [{"expr": "node_load1", "datasource": {"type": "prometheus"}}]}
	    ]
	  }
	}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer ts.Close()

	c, _ := New(ts.URL)
	dash, _ := c.Dashboard(context.Background(), "x")
	got := dash.Queries()
	sort.Strings(got)
	want := []string{"node_load1", "up"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Queries mismatch (-want +got):\n%s", diff)
	}
}

func TestDashboard_Queries_NonPrometheusDatasourceSkipped(t *testing.T) {
	body := `{
	  "dashboard": {
	    "uid": "y",
	    "title": "Mixed",
	    "panels": [
	      {"type": "logs", "targets": [{"expr": "{job=\"api\"}", "datasource": {"type": "loki"}}]},
	      {"type": "graph", "targets": [{"expr": "up", "datasource": {"type": "prometheus"}}]}
	    ]
	  }
	}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer ts.Close()

	c, _ := New(ts.URL)
	dash, _ := c.Dashboard(context.Background(), "y")
	got := dash.Queries()
	want := []string{"up"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Queries mismatch (-want +got):\n%s", diff)
	}
}

func TestDashboard_PanelTargets_WalksRecursivelyAndFiltersNonProm(t *testing.T) {
	body := `{"dashboard":{"uid":"d1","title":"Top","panels":[
		{"type":"graph","title":"P1","targets":[{"expr":"up","datasource":{"type":"prometheus"}}]},
		{"type":"row","title":"Row","panels":[
			{"type":"graph","title":"P2","targets":[{"expr":"down","datasource":{"type":"prometheus"}}]},
			{"type":"logs","title":"P3","targets":[{"expr":"{job=\"app\"}","datasource":{"type":"loki"}}]}
		]},
		{"type":"graph","title":"","targets":[{"expr":"node_load1","datasource":{"type":"prometheus"}}]}
	]}}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer ts.Close()
	c, err := New(ts.URL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	dash, err := c.Dashboard(context.Background(), "d1")
	if err != nil {
		t.Fatalf("Dashboard: %v", err)
	}
	got := dash.PanelTargets()
	// Recursion mirrors collectPanelQueries: for each top-level panel,
	// descend into its children before processing its own targets. So
	// P1's own targets emit before we descend into the Row sibling.
	want := []PanelTarget{
		{PanelTitle: "P1", Expr: "up"},
		{PanelTitle: "P2", Expr: "down"}, // nested under Row; P3 (loki) skipped
		{PanelTitle: "<untitled panel>", Expr: "node_load1"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("PanelTargets mismatch (-want +got):\n%s", diff)
	}
}

func TestDashboard_Queries_TargetWithoutExpr(t *testing.T) {
	body := `{
	  "dashboard": {
	    "uid": "z",
	    "title": "Edges",
	    "panels": [
	      {"type": "graph", "targets": [
	        {"datasource": {"type": "prometheus"}},
	        {"expr": "", "datasource": {"type": "prometheus"}},
	        {"expr": "up", "datasource": {"type": "prometheus"}}
	      ]}
	    ]
	  }
	}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer ts.Close()

	c, _ := New(ts.URL)
	dash, _ := c.Dashboard(context.Background(), "z")
	got := dash.Queries()
	want := []string{"up"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Queries mismatch (-want +got):\n%s", diff)
	}
}
