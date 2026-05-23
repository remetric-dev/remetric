// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

//go:build e2e

package e2e

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestE2E_DashboardsBroken_JSON verifies the dashboards broken subcommand
// surfaces the broken panel provisioned by e2e/grafana/dashboards/broken-panel-demo.json,
// which references a metric Prometheus does not scrape.
func TestE2E_DashboardsBroken_JSON(t *testing.T) {
	out, err := runCmd(t, binPath(t),
		"dashboards", "broken",
		"--prometheus", "http://localhost:9090",
		"--grafana", "http://localhost:3000",
		"--output", "json",
	)
	if err != nil {
		t.Fatalf("dashboards broken failed: %v\noutput:\n%s", err, out)
	}

	var payload struct {
		Findings []struct {
			Class     string `json:"class"`
			Metric    string `json:"metric"`
			Dashboard string `json:"dashboard"`
		} `json:"findings"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("json.Unmarshal: %v\noutput:\n%s", err, out)
	}
	if len(payload.Findings) == 0 {
		t.Fatalf("expected at least 1 finding; got 0\noutput:\n%s", out)
	}
	found := false
	for _, f := range payload.Findings {
		if f.Class == "broken-panel" && f.Metric == "nonexistent_broken_panel_demo_xyz_total" {
			if !strings.Contains(f.Dashboard, "Broken Panel Demo") {
				t.Errorf("Dashboard field = %q, want %q", f.Dashboard, "Broken Panel Demo")
			}
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected broken-panel finding for nonexistent_broken_panel_demo_xyz_total; got %+v", payload.Findings)
	}
}
