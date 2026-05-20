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
	if err := renderFindings(cfg, &buf, fs); err != nil {
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
	if err := renderFindings(cfg, &buf, fs); err != nil {
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
	err := renderFindings(cfg, &buf, nil)
	if err == nil {
		t.Errorf("expected error for unsupported output, got nil")
	}
}
