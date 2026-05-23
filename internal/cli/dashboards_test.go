// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package cli_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cli "github.com/remetric-dev/remetric/internal/cli"
)

// newDashboardsStubs spins up a Prom + Grafana pair where one dashboard
// references a missing metric "missing_xyz".
func newDashboardsStubs(t *testing.T) (promURL, grafURL string) {
	t.Helper()
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/label/__name__/values":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": []string{"up"}})
		case "/api/v1/rules":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   map[string]any{"groups": []map[string]any{}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(prom.Close)

	graf := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/search":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"uid": "d1", "title": "Disk", "url": "/d/d1/disk"},
			})
		case "/api/dashboards/uid/d1":
			_, _ = w.Write([]byte(`{"dashboard":{"uid":"d1","title":"Disk","panels":[
				{"type":"graph","title":"P","targets":[{"expr":"missing_xyz","datasource":{"type":"prometheus"}}]}
			]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(graf.Close)
	return prom.URL, graf.URL
}

func TestDashboardsBroken_RequiresGrafana(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args:    []string{"dashboards", "broken", "--prometheus", "http://127.0.0.1:1"},
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	if code != 2 {
		t.Errorf("exit code = %d, want 2 (flag error); stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "--grafana is required") {
		t.Errorf("stderr = %q, want contains '--grafana is required'", stderr.String())
	}
}

func TestDashboardsBroken_TextOutput(t *testing.T) {
	promURL, grafURL := newDashboardsStubs(t)
	var stdout, stderr bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args: []string{
			"dashboards", "broken",
			"--prometheus", promURL,
			"--grafana", grafURL,
		},
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "missing_xyz") {
		t.Errorf("stdout should mention missing_xyz; got: %s", stdout.String())
	}
}

func TestDashboardsBroken_JSONOutput(t *testing.T) {
	promURL, grafURL := newDashboardsStubs(t)
	var stdout, stderr bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args: []string{
			"dashboards", "broken",
			"--prometheus", promURL,
			"--grafana", grafURL,
			"--output", "json",
		},
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	var payload struct {
		Findings []struct {
			Class     string `json:"class"`
			Metric    string `json:"metric"`
			Dashboard string `json:"dashboard"`
		} `json:"findings"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal: %v\nstdout=%s", err, stdout.String())
	}
	if len(payload.Findings) != 1 {
		t.Fatalf("findings len = %d, want 1; payload=%+v", len(payload.Findings), payload)
	}
	if payload.Findings[0].Class != "broken-panel" || payload.Findings[0].Metric != "missing_xyz" {
		t.Errorf("finding = %+v, want class=broken-panel metric=missing_xyz", payload.Findings[0])
	}
	if payload.Findings[0].Dashboard != "Disk" {
		t.Errorf("Dashboard = %q, want %q", payload.Findings[0].Dashboard, "Disk")
	}
}

func TestDashboardsBroken_IgnoreDashboardDrops(t *testing.T) {
	promURL, grafURL := newDashboardsStubs(t)
	var stdout, stderr bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args: []string{
			"dashboards", "broken",
			"--prometheus", promURL,
			"--grafana", grafURL,
			"--ignore-dashboard", "Disk",
			"--output", "json",
		},
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%s", code, stderr.String())
	}
	var payload struct {
		Findings     []any `json:"findings"`
		IgnoredCount int   `json:"ignored_count"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &payload); err != nil {
		t.Fatalf("json.Unmarshal: %v\nstdout=%s", err, stdout.String())
	}
	if len(payload.Findings) != 0 {
		t.Errorf("findings len = %d, want 0 (dropped by --ignore-dashboard)", len(payload.Findings))
	}
	if payload.IgnoredCount != 1 {
		t.Errorf("IgnoredCount = %d, want 1", payload.IgnoredCount)
	}
}
