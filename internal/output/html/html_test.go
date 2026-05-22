// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package html_test

import (
	"bytes"
	"flag"
	"os"
	"strings"
	"testing"
	"time"

	xhtml "golang.org/x/net/html"

	"github.com/remetric-dev/remetric/internal/findings"
	"github.com/remetric-dev/remetric/internal/output/html"
)

var update = flag.Bool("update", false, "rewrite golden files")

func sampleReport() *findings.Report {
	return &findings.Report{
		Version:   "v0.1.0",
		ScannedAt: time.Date(2026, 5, 21, 12, 0, 0, 0, time.UTC),
		Target:    &findings.Target{PrometheusURL: "http://prom:9090", PrometheusVersion: "2.51.2"},
		Overview:  &findings.Overview{TotalSeries: 1_234_567, TotalMetrics: 4321},
		Warnings:  []string{"vmalert not configured"},
		Findings: []findings.Finding{
			{
				ID: "alert_hygiene/always_firing", Severity: findings.SeverityCritical,
				Category: findings.CategoryAlertHygiene,
				Title:    "Alert Broken firing 100.0% of window",
				Evidence: findings.Evidence{Description: "firing 100% of 168h0m0s"},
				Fix:      findings.Fix{Type: "rule_change", Config: "- alert: Broken\n  expr: vector(1)"},
			},
			{
				ID: "alert_hygiene/never_fired", Severity: findings.SeverityMedium,
				Category: findings.CategoryAlertHygiene,
				Title:    "Alert Noise never fired in 168h0m0s",
				Evidence: findings.Evidence{Description: "no firing samples"},
				Fix:      findings.Fix{Type: "rule_change", Config: "- alert: Noise\n  expr: vector(0)"},
			},
			{
				ID: "cardinality/high_cardinality_label", Severity: findings.SeverityHigh,
				Category: findings.CategoryCardinality,
				Title:    "http_requests_total: 250k series",
				Evidence: findings.Evidence{Description: "user_id unbounded"},
				Fix:      findings.Fix{Type: "drop_label", Config: "- action: labeldrop\n  regex: user_id"},
			},
			{
				ID: "label_patterns/unbounded", Severity: findings.SeverityLow,
				Category: findings.CategoryLabelPatterns,
				Title:    "Label trace_id looks unbounded",
				Evidence: findings.Evidence{Description: "1M unique values across 5 metrics"},
				Fix:      findings.Fix{Type: "drop_label", Config: "- action: labeldrop\n  regex: trace_id"},
			},
		},
		Summary: findings.Summary{BySeverity: map[string]int{"critical": 1, "high": 1, "medium": 1, "low": 1}},
	}
}

func TestHTMLReport_ValidMarkup(t *testing.T) {
	var buf bytes.Buffer
	if err := html.New(&buf).RenderReport(sampleReport()); err != nil {
		t.Fatalf("RenderReport: %v", err)
	}
	out := buf.String()
	if _, err := xhtml.Parse(bytes.NewReader(buf.Bytes())); err != nil {
		t.Errorf("HTML parse error: %v", err)
	}
	if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(out)), "<!doctype html>") {
		t.Errorf("output missing <!DOCTYPE html> prefix")
	}
}

func TestHTMLReport_RendersAllSeverities(t *testing.T) {
	var buf bytes.Buffer
	if err := html.New(&buf).RenderReport(sampleReport()); err != nil {
		t.Fatalf("RenderReport: %v", err)
	}
	got := buf.String()
	for _, sev := range []string{"sev-CRITICAL", "sev-HIGH", "sev-MEDIUM", "sev-LOW"} {
		if !strings.Contains(got, sev) {
			t.Errorf("output missing class %q", sev)
		}
	}
	if !strings.Contains(got, "<svg") {
		t.Errorf("output missing inline SVG donut")
	}
}

func TestHTMLReport_Golden(t *testing.T) {
	var buf bytes.Buffer
	if err := html.New(&buf).RenderReport(sampleReport()); err != nil {
		t.Fatalf("RenderReport: %v", err)
	}
	goldenPath := "testdata/full-report.html.golden"
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

func TestRenderReport_HTML_IgnoredCountInSummary(t *testing.T) {
	rep := &findings.Report{
		Version:      "1.0",
		ScannedAt:    time.Unix(1700000000, 0).UTC(),
		Summary:      findings.Summary{BySeverity: map[string]int{"critical": 0, "high": 0, "medium": 0, "low": 0}},
		IgnoredCount: 4,
	}
	var buf bytes.Buffer
	if err := html.New(&buf).RenderReport(rep); err != nil {
		t.Fatalf("RenderReport: %v", err)
	}
	if !strings.Contains(buf.String(), "Ignored") || !strings.Contains(buf.String(), "4") {
		t.Errorf("expected 'Ignored' row with count 4 in HTML:\n%s", buf.String())
	}
}

func TestRenderReport_HTML_IgnoredCountZeroHidden(t *testing.T) {
	rep := &findings.Report{
		Version:      "1.0",
		ScannedAt:    time.Unix(1700000000, 0).UTC(),
		Summary:      findings.Summary{BySeverity: map[string]int{"critical": 0, "high": 0, "medium": 0, "low": 0}},
		IgnoredCount: 0,
	}
	var buf bytes.Buffer
	if err := html.New(&buf).RenderReport(rep); err != nil {
		t.Fatalf("RenderReport: %v", err)
	}
	if strings.Contains(buf.String(), ">Ignored<") {
		t.Errorf("IgnoredCount=0 should be hidden: %s", buf.String())
	}
}
