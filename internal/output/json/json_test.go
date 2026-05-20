// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package json

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/remetric-dev/remetric/internal/findings"
)

var update = flag.Bool("update", false, "update golden files")

func goldenPath(name string) string { return filepath.Join("testdata", name) }

func TestRenderFindings_Golden(t *testing.T) {
	fs := []findings.Finding{
		{
			ID:       "label-user_id",
			Severity: findings.SeverityHigh,
			Category: findings.CategoryLabelPatterns,
			Title:    `suspicious label "user_id" with 5000 unique values across 1 metrics`,
			Evidence: findings.Evidence{
				Label:        "user_id",
				UniqueValues: 5000,
				SampleValues: []string{"u-1", "u-2"},
				SeriesCount:  800_000,
				Description:  "user_id has 5000 unique values across 1 metrics",
			},
			Fix: findings.Fix{
				Type:   "drop_label",
				Config: "metric_relabel_configs:\n  - regex: 'user_id'\n    action: labeldrop\n",
			},
			Impact: findings.Impact{
				SeriesReduction:  799_840,
				Percentage:       99.98,
				EstimationMethod: "labeldrop_upper_bound",
			},
		},
	}
	var buf bytes.Buffer
	if err := New(&buf).RenderFindings(fs); err != nil {
		t.Fatalf("RenderFindings: %v", err)
	}
	checkGolden(t, "findings.json", buf.Bytes())
}

func TestRenderReport_Golden(t *testing.T) {
	rep := &findings.Report{
		Version:   "0.2.0",
		ScannedAt: time.Date(2026, 5, 20, 10, 23, 45, 0, time.UTC),
		Target: &findings.Target{
			PrometheusURL:     "http://prometheus.example.com:9090",
			PrometheusVersion: "2.51.2",
		},
		Overview: &findings.Overview{
			TotalSeries:          2_341_892,
			TotalMetrics:         4_127,
			EstimatedMemoryBytes: 13_314_752_512,
		},
		Findings: []findings.Finding{},
		Summary: findings.Summary{
			BySeverity:               map[string]int{},
			PotentialSeriesReduction: 0,
		},
	}
	var buf bytes.Buffer
	if err := New(&buf).RenderReport(rep); err != nil {
		t.Fatalf("RenderReport: %v", err)
	}
	checkGolden(t, "report.json", buf.Bytes())
}

func TestRenderLabelInventory_Golden(t *testing.T) {
	inv := LabelInventory{
		Metric: "http_requests_total",
		Labels: []LabelStat{
			{Name: "user_id", UniqueValues: 5000, SampleValues: []string{"u-1"}},
			{Name: "status", UniqueValues: 5},
		},
	}
	var buf bytes.Buffer
	if err := New(&buf).RenderLabelInventory(inv); err != nil {
		t.Fatalf("RenderLabelInventory: %v", err)
	}
	checkGolden(t, "inventory.json", buf.Bytes())
}

func checkGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	path := goldenPath(name)
	if *update {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update to create)", path, err)
	}
	if diff := cmp.Diff(string(want), string(got)); diff != "" {
		t.Errorf("%s mismatch (-want +got):\n%s", name, diff)
	}
}
