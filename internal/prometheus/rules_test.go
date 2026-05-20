// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package prometheus_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"

	prom "github.com/remetric-dev/remetric/internal/prometheus"
)

func TestRules_ParsesAlertAndRecording(t *testing.T) {
	body := `{
	  "status": "success",
	  "data": {
	    "groups": [
	      {
	        "name": "alerts",
	        "file": "/etc/prom/alerts.yml",
	        "rules": [
	          {"name": "HighErrors", "query": "rate(errors_total[5m]) > 0", "type": "alerting"},
	          {"name": "job:errors_total:rate5m", "query": "rate(errors_total[5m])", "type": "recording"}
	        ]
	      }
	    ]
	  }
	}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/rules" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer ts.Close()

	c, err := prom.New(ts.URL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	got, err := c.Rules(context.Background())
	if err != nil {
		t.Fatalf("Rules: %v", err)
	}
	want := &prom.Rules{
		Groups: []prom.RuleGroup{
			{
				Name: "alerts",
				File: "/etc/prom/alerts.yml",
				Rules: []prom.Rule{
					{Name: "HighErrors", Query: "rate(errors_total[5m]) > 0", Type: "alerting"},
					{Name: "job:errors_total:rate5m", Query: "rate(errors_total[5m])", Type: "recording"},
				},
			},
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Rules mismatch (-want +got):\n%s", diff)
	}
}

func TestRules_EmptyResponse(t *testing.T) {
	body := `{"status":"success","data":{"groups":[]}}`
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	defer ts.Close()
	c, _ := prom.New(ts.URL)
	got, err := c.Rules(context.Background())
	if err != nil {
		t.Fatalf("Rules: %v", err)
	}
	if len(got.Groups) != 0 {
		t.Errorf("len groups = %d, want 0", len(got.Groups))
	}
}
