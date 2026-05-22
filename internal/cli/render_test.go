// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/remetric-dev/remetric/internal/config"
	"github.com/remetric-dev/remetric/internal/findings"
)

func TestRenderFindings_Terminal(t *testing.T) {
	fs := []findings.Finding{
		{ID: "x", Severity: findings.SeverityHigh, Category: findings.CategoryCardinality, Metric: "m", Evidence: findings.Evidence{Label: "l", SeriesCount: 1000, UniqueValues: 50}},
	}
	var buf bytes.Buffer
	cfg := &config.Config{Output: "terminal", NoColor: true}
	if err := renderFindings(cfg, &buf, fs, 0); err != nil {
		t.Fatalf("renderFindings: %v", err)
	}
	if !strings.Contains(strings.ToUpper(buf.String()), "SEVERITY") {
		t.Errorf("terminal output missing SEVERITY header:\n%s", buf.String())
	}
}

func TestRenderFindings_JSON(t *testing.T) {
	fs := []findings.Finding{
		{ID: "x", Severity: findings.SeverityHigh, Category: findings.CategoryCardinality},
	}
	var buf bytes.Buffer
	cfg := &config.Config{Output: "json"}
	if err := renderFindings(cfg, &buf, fs, 0); err != nil {
		t.Fatalf("renderFindings: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, `"findings"`) {
		t.Errorf("JSON output missing findings key:\n%s", out)
	}
	if !strings.Contains(out, `"severity": "high"`) {
		t.Errorf("JSON output missing severity:\n%s", out)
	}
}

func TestRenderFindings_InvalidOutput(t *testing.T) {
	var buf bytes.Buffer
	cfg := &config.Config{Output: "html"}
	err := renderFindings(cfg, &buf, nil, 0)
	if err == nil {
		t.Errorf("expected error for unsupported output, got nil")
	}
}

func TestRenderReport_PrintsWarnings_Terminal(t *testing.T) {
	r := findings.NewReport("1.0", nil)
	r.Warnings = []string{"vmalert URL not configured"}
	var buf bytes.Buffer
	cfg := &config.Config{Output: "terminal", NoColor: true}
	if err := renderReport(cfg, &buf, r); err != nil {
		t.Fatalf("renderReport: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "warning") {
		t.Errorf("expected 'warning' in output: %q", out)
	}
	if !strings.Contains(out, "vmalert URL not configured") {
		t.Errorf("expected warning text in output: %q", out)
	}
}

func TestRenderReport_NoWarningsOmitsBanner_Terminal(t *testing.T) {
	r := findings.NewReport("1.0", nil)
	var buf bytes.Buffer
	cfg := &config.Config{Output: "terminal", NoColor: true}
	if err := renderReport(cfg, &buf, r); err != nil {
		t.Fatalf("renderReport: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "warning") {
		t.Errorf("did not expect 'warning' in output for empty Warnings: %q", out)
	}
}

func TestWriteTerminalIgnored_SingularPlural(t *testing.T) {
	tests := []struct {
		n    int
		want string
	}{
		{1, "ignored: 1 finding (dropped by --ignore-*)"},
		{2, "ignored: 2 findings (dropped by --ignore-*)"},
	}
	for _, tc := range tests {
		var buf bytes.Buffer
		if err := writeTerminalIgnored(&buf, tc.n); err != nil {
			t.Fatalf("n=%d: %v", tc.n, err)
		}
		if !strings.Contains(buf.String(), tc.want) {
			t.Errorf("n=%d: got %q, want substring %q", tc.n, buf.String(), tc.want)
		}
	}
}

func TestRenderFormat_Terminal_PrintsIgnoredFooter(t *testing.T) {
	r := findings.NewReport("1.0", nil)
	r.IgnoredCount = 2
	var buf bytes.Buffer
	if err := renderFormat(&buf, "terminal", r, true); err != nil {
		t.Fatalf("renderFormat: %v", err)
	}
	if !strings.Contains(buf.String(), "ignored: 2 findings") {
		t.Errorf("expected ignored footer, got:\n%s", buf.String())
	}
}

func TestRenderFormat_Terminal_SingularIgnoredFooter(t *testing.T) {
	r := findings.NewReport("1.0", nil)
	r.IgnoredCount = 1
	var buf bytes.Buffer
	if err := renderFormat(&buf, "terminal", r, true); err != nil {
		t.Fatalf("renderFormat: %v", err)
	}
	if !strings.Contains(buf.String(), "ignored: 1 finding (dropped by --ignore-*)") {
		t.Errorf("expected singular ignored footer, got:\n%s", buf.String())
	}
	if strings.Contains(buf.String(), "1 findings") {
		t.Errorf("should not pluralise on n=1:\n%s", buf.String())
	}
}
