// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package markdown_test

import (
	"bytes"
	"flag"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/remetric-dev/remetric/internal/findings"
	"github.com/remetric-dev/remetric/internal/output/markdown"
)

var update = flag.Bool("update", false, "rewrite golden files")

func TestRenderer_FullReport_Golden(t *testing.T) {
	rep := &findings.Report{
		Version:   "v0.1.0",
		ScannedAt: time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC),
		Target:    &findings.Target{PrometheusURL: "http://prom:9090", PrometheusVersion: "2.51.2"},
		Overview:  &findings.Overview{TotalSeries: 1_234_567, TotalMetrics: 4321},
		Warnings:  []string{"vmalert not configured"},
		Findings: []findings.Finding{
			{
				ID:       "alert_hygiene/never_fired",
				Severity: findings.SeverityMedium,
				Category: findings.CategoryAlertHygiene,
				Title:    "Alert NoiseAlert never fired in 168h0m0s",
				Evidence: findings.Evidence{Description: "no firing samples in 168h0m0s lookback"},
				Fix:      findings.Fix{Type: "rule_change", Config: "- alert: NoiseAlert\n  expr: up == 0"},
			},
			{
				ID:       "cardinality/high_cardinality_label",
				Severity: findings.SeverityCritical,
				Category: findings.CategoryCardinality,
				Title:    "http_requests_total: 250k series",
				Metric:   "http_requests_total",
				Evidence: findings.Evidence{Label: "user_id", UniqueValues: 1_000_000, SeriesCount: 250_000, Description: "user_id is unbounded"},
				Fix:      findings.Fix{Type: "drop_label", Config: "- action: labeldrop\n  regex: user_id"},
				Impact:   findings.Impact{SeriesReduction: 240_000, Percentage: 96.0},
			},
		},
		Summary: findings.Summary{
			BySeverity:               map[string]int{"critical": 1, "medium": 1},
			PotentialSeriesReduction: 240_000,
		},
	}

	var buf bytes.Buffer
	if err := markdown.New(&buf).RenderReport(rep); err != nil {
		t.Fatalf("RenderReport: %v", err)
	}

	goldenPath := "testdata/full-report.md.golden"
	if *update {
		if err := os.WriteFile(goldenPath, buf.Bytes(), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden: %v", err)
	}
	if string(want) != buf.String() {
		t.Errorf("output mismatch (run with -update to refresh):\nGOT:\n%s\nWANT:\n%s", buf.String(), string(want))
	}
}

func TestRenderReport_IgnoredCountInSummary(t *testing.T) {
	rep := &findings.Report{
		Version:      "1.0",
		ScannedAt:    time.Unix(1700000000, 0).UTC(),
		Findings:     nil,
		Summary:      findings.Summary{BySeverity: map[string]int{}},
		IgnoredCount: 3,
	}
	var buf bytes.Buffer
	if err := markdown.New(&buf).RenderReport(rep); err != nil {
		t.Fatalf("RenderReport: %v", err)
	}
	if !strings.Contains(buf.String(), "**Ignored:** 3 findings") {
		t.Errorf("expected ignored line in summary, got:\n%s", buf.String())
	}
}

func TestRenderReport_IgnoredCountSingular(t *testing.T) {
	rep := &findings.Report{
		Version:      "1.0",
		ScannedAt:    time.Unix(1700000000, 0).UTC(),
		Findings:     nil,
		Summary:      findings.Summary{BySeverity: map[string]int{}},
		IgnoredCount: 1,
	}
	var buf bytes.Buffer
	if err := markdown.New(&buf).RenderReport(rep); err != nil {
		t.Fatalf("RenderReport: %v", err)
	}
	if !strings.Contains(buf.String(), "**Ignored:** 1 finding\n") {
		t.Errorf("expected singular 'finding', got:\n%s", buf.String())
	}
	if strings.Contains(buf.String(), "1 findings") {
		t.Errorf("should not pluralise on n=1:\n%s", buf.String())
	}
}

func TestRenderReport_IgnoredCountZeroHidden(t *testing.T) {
	rep := &findings.Report{Version: "1.0", ScannedAt: time.Unix(1700000000, 0).UTC(), Summary: findings.Summary{BySeverity: map[string]int{}}, IgnoredCount: 0}
	var buf bytes.Buffer
	if err := markdown.New(&buf).RenderReport(rep); err != nil {
		t.Fatalf("RenderReport: %v", err)
	}
	if strings.Contains(buf.String(), "Ignored:") {
		t.Errorf("IgnoredCount=0 should be hidden: %s", buf.String())
	}
}
