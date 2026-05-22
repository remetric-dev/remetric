// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package terminal

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/remetric-dev/remetric/internal/findings"
)

var update = flag.Bool("update", false, "update golden files")

func goldenAssert(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("testdata", name)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("write golden: %v", err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	if string(want) != got {
		t.Errorf("output mismatch for %s. Run with -update to refresh.\n--- want ---\n%s\n--- got ---\n%s", name, want, got)
	}
}

func sampleFindings() []findings.Finding {
	return []findings.Finding{
		{
			ID:       "card-istio_requests_total-destination_principal",
			Severity: findings.SeverityCritical, Category: findings.CategoryCardinality,
			Class:  findings.ClassHotLabel,
			Title:  "high cardinality in istio_requests_total due to label \"destination_principal\"",
			Metric: "istio_requests_total",
			Evidence: findings.Evidence{
				Label: "destination_principal", UniqueValues: 5234,
				SampleValues: []string{"spiffe://orders", "spiffe://payments"},
				SeriesCount:  487234,
				Description:  "destination_principal has 5234 unique values inside istio_requests_total",
			},
			Fix:    findings.Fix{Type: "scrape_config", Config: "metric_relabel_configs:\n  - source_labels: [__name__]\n    regex: \"istio_requests_total\"\n    action: keep\n  - regex: \"destination_principal\"\n    action: labeldrop\n"},
			Impact: findings.Impact{SeriesReduction: 487141, Percentage: 99.98, EstimationMethod: "labeldrop_upper_bound"},
		},
		{
			ID:       "card-etcd_request_duration_seconds_bucket-grpc_method",
			Severity: findings.SeverityHigh, Category: findings.CategoryCardinality,
			Class:  findings.ClassHotLabel,
			Title:  "high cardinality in etcd_request_duration_seconds_bucket due to label \"grpc_method\"",
			Metric: "etcd_request_duration_seconds_bucket",
			Evidence: findings.Evidence{
				Label: "grpc_method", UniqueValues: 148, SampleValues: []string{"foo", "bar"},
				SeriesCount: 124540, Description: "grpc_method has 148 unique values",
			},
			Fix:    findings.Fix{Type: "scrape_config", Config: "metric_relabel_configs:\n  - source_labels: [__name__]\n    regex: \"etcd_request_duration_seconds_bucket\"\n    action: keep\n  - regex: \"grpc_method\"\n    action: labeldrop\n"},
			Impact: findings.Impact{SeriesReduction: 123699, Percentage: 99.32, EstimationMethod: "labeldrop_upper_bound"},
		},
		{
			ID:       "card-node_filesystem_size_bytes-device",
			Severity: findings.SeverityMedium, Category: findings.CategoryCardinality,
			Class:  findings.ClassHotLabel,
			Title:  "high cardinality in node_filesystem_size_bytes due to label \"device\"",
			Metric: "node_filesystem_size_bytes",
			Evidence: findings.Evidence{
				Label: "device", UniqueValues: 52, SampleValues: []string{"sda1", "sda2"},
				SeriesCount: 31200,
			},
			Impact: findings.Impact{SeriesReduction: 30600, Percentage: 98.0, EstimationMethod: "labeldrop_upper_bound"},
		},
	}
}

func TestRenderFindings_AllSeverities(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, WithColor(false), WithWidth(100))
	if err := r.RenderFindings(sampleFindings()); err != nil {
		t.Fatalf("RenderFindings err = %v", err)
	}
	goldenAssert(t, "findings_all_severities.golden", buf.String())
}

func TestRenderFindings_EmptyIsNoOp(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, WithColor(false), WithWidth(100))
	if err := r.RenderFindings(nil); err != nil {
		t.Fatalf("RenderFindings: %v", err)
	}
	if buf.Len() != 0 {
		t.Errorf("empty input should write nothing, got:\n%q", buf.String())
	}
}

func TestRenderFindings_FixBlockOnlyForCritHigh(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, WithColor(false), WithWidth(100))
	if err := r.RenderFindings(sampleFindings()); err != nil {
		t.Fatalf("err = %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "istio_requests_total") || !strings.Contains(out, "grpc_method") {
		t.Errorf("missing crit/high fix blocks")
	}
	if strings.Contains(out, "device\n    action: labeldrop") {
		t.Errorf("medium row should NOT have a fix block")
	}
}

func TestRenderFindings_ClassColumnAndAlertFallback(t *testing.T) {
	fs := []findings.Finding{
		{
			ID:       "card-app_requests_total-trace_id",
			Severity: findings.SeverityCritical,
			Category: findings.CategoryCardinality,
			Class:    findings.ClassHotLabel,
			Metric:   "app_requests_total",
			Evidence: findings.Evidence{Label: "trace_id", UniqueValues: 500, SeriesCount: 500},
			Impact:   findings.Impact{SeriesReduction: 499},
		},
		{
			ID:       "alert_hygiene/never_fired",
			Severity: findings.SeverityMedium,
			Category: findings.CategoryAlertHygiene,
			Class:    findings.ClassNeverFiringAlert,
			Alert:    "HighRequestLatencyP99",
			// Metric and Evidence.Label intentionally empty - alert
			// findings live in their own entity space.
		},
	}

	var buf bytes.Buffer
	r := New(&buf, WithColor(false))
	if err := r.RenderFindings(fs); err != nil {
		t.Fatalf("RenderFindings: %v", err)
	}
	out := buf.String()

	// Class column header rendered.
	if !strings.Contains(out, "CLASS") {
		t.Errorf("expected CLASS column header in output:\n%s", out)
	}
	// Class slug appears in the cardinality row.
	if !strings.Contains(out, "hot-label") {
		t.Errorf("expected 'hot-label' class slug in row:\n%s", out)
	}
	// Class slug appears in the alert row.
	if !strings.Contains(out, "never-firing-alert") {
		t.Errorf("expected 'never-firing-alert' class slug in row:\n%s", out)
	}
	// Alert name renders in the Metric column (fallback).
	if !strings.Contains(out, "HighRequestLatencyP99") {
		t.Errorf("expected alert name in Metric column (fallback when Metric empty):\n%s", out)
	}
}

func TestRenderFindings_LabelPatternFixBlock(t *testing.T) {
	f := findings.Finding{
		ID:       "label-user_id",
		Severity: findings.SeverityHigh,
		Category: findings.CategoryLabelPatterns,
		Title:    "suspicious label user_id",
		// Metric intentionally empty.
		Evidence: findings.Evidence{
			Label:        "user_id",
			UniqueValues: 5000,
			SampleValues: []string{"u-1", "u-2"},
			SeriesCount:  100_000,
			Description:  "user_id has 5000 unique values across 3 metrics",
		},
		Fix: findings.Fix{
			Type:   "drop_label",
			Config: "metric_relabel_configs:\n  - regex: 'user_id'\n    action: labeldrop\n",
		},
		Impact: findings.Impact{SeriesReduction: 80_000, Percentage: 80, EstimationMethod: "labeldrop_upper_bound"},
	}

	var buf bytes.Buffer
	r := New(&buf, WithColor(false))
	if err := r.RenderFindings([]findings.Finding{f}); err != nil {
		t.Fatalf("RenderFindings: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "label 'user_id' has 5,000 unique values") {
		t.Errorf("expected label-style heading, got:\n%s", out)
	}
}
