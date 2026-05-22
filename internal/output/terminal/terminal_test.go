// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package terminal

import (
	"bytes"
	"strings"
	"testing"

	"github.com/remetric-dev/remetric/internal/findings"
)

func TestRenderer_NoColorViaOption(t *testing.T) {
	r := New(&bytes.Buffer{}, WithColor(false))
	if !r.noColor {
		t.Errorf("noColor = false, want true")
	}
}

func TestRenderer_AutoNoColorOnNonTTY(t *testing.T) {
	r := New(&bytes.Buffer{})
	if !r.noColor {
		t.Errorf("noColor for buffer writer = false, want true (non-TTY)")
	}
}

func TestRenderer_RespectsNoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	r := New(&bytes.Buffer{}, WithColor(true))
	if !r.noColor {
		t.Errorf("with NO_COLOR=1, noColor = false, want true")
	}
}

func TestRenderFindings_ReferenceLineWhenDocURLSet(t *testing.T) {
	f := findings.Finding{
		ID:       "card-istio_requests_total-destination_principal",
		Severity: findings.SeverityCritical,
		Category: findings.CategoryCardinality,
		Class:    findings.ClassHotLabel,
		DocURL:   "https://remetric.dev/findings/hot-label",
		Title:    "high cardinality",
		Metric:   "istio_requests_total",
		Evidence: findings.Evidence{
			Label:        "destination_principal",
			UniqueValues: 5234,
			SeriesCount:  487234,
		},
		Fix: findings.Fix{Type: "scrape_config", Config: "action: labeldrop\n"},
	}
	var buf bytes.Buffer
	r := New(&buf, WithColor(false), WithWidth(100))
	if err := r.RenderFindings([]findings.Finding{f}); err != nil {
		t.Fatalf("RenderFindings: %v", err)
	}
	want := "Reference: https://remetric.dev/findings/hot-label"
	if !strings.Contains(buf.String(), want) {
		t.Errorf("output missing %q, got:\n%s", want, buf.String())
	}
}

func TestRenderFindings_NoReferenceLineWhenDocURLEmpty(t *testing.T) {
	f := findings.Finding{
		ID:       "card-istio_requests_total-destination_principal",
		Severity: findings.SeverityCritical,
		Category: findings.CategoryCardinality,
		Title:    "high cardinality",
		Metric:   "istio_requests_total",
		Evidence: findings.Evidence{
			Label:        "destination_principal",
			UniqueValues: 5234,
			SeriesCount:  487234,
		},
		Fix: findings.Fix{Type: "scrape_config", Config: "action: labeldrop\n"},
	}
	var buf bytes.Buffer
	r := New(&buf, WithColor(false), WithWidth(100))
	if err := r.RenderFindings([]findings.Finding{f}); err != nil {
		t.Fatalf("RenderFindings: %v", err)
	}
	if strings.Contains(buf.String(), "Reference:") {
		t.Errorf("output should not contain 'Reference:' when DocURL is empty, got:\n%s", buf.String())
	}
}
