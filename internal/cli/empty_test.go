// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package cli

import (
	"bytes"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/remetric-dev/remetric/internal/config"
	"github.com/remetric-dev/remetric/internal/findings"
)

func TestTallyBySeverity(t *testing.T) {
	fs := []findings.Finding{
		{Severity: findings.SeverityLow},
		{Severity: findings.SeverityLow},
		{Severity: findings.SeverityMedium},
		{Severity: findings.SeverityCritical},
	}
	got := tallyBySeverity(fs)
	want := map[findings.Severity]int{
		findings.SeverityLow:      2,
		findings.SeverityMedium:   1,
		findings.SeverityCritical: 1,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("tallyBySeverity mismatch (-want +got):\n%s", diff)
	}
}

func TestTallyBySeverity_Empty(t *testing.T) {
	got := tallyBySeverity(nil)
	if len(got) != 0 {
		t.Errorf("len = %d, want 0", len(got))
	}
}

func TestFilterAtLeast(t *testing.T) {
	fs := []findings.Finding{
		{ID: "a", Severity: findings.SeverityLow},
		{ID: "b", Severity: findings.SeverityMedium},
		{ID: "c", Severity: findings.SeverityHigh},
		{ID: "d", Severity: findings.SeverityCritical},
	}
	got := filterAtLeast(fs, findings.SeverityHigh)
	if len(got) != 2 {
		t.Fatalf("got %d findings, want 2: %+v", len(got), got)
	}
	gotIDs := []string{got[0].ID, got[1].ID}
	want := []string{"c", "d"}
	if diff := cmp.Diff(want, gotIDs); diff != "" {
		t.Errorf("filterAtLeast mismatch (-want +got):\n%s", diff)
	}
}

func TestFilterAtLeast_KeepsAllWhenLow(t *testing.T) {
	fs := []findings.Finding{
		{Severity: findings.SeverityLow},
		{Severity: findings.SeverityCritical},
	}
	got := filterAtLeast(fs, findings.SeverityLow)
	if len(got) != 2 {
		t.Errorf("filterAtLeast(low) dropped findings: got %d, want 2", len(got))
	}
}

func TestEmptyCopy_Values(t *testing.T) {
	if cardinalityCopy.Subject == "" {
		t.Error("cardinalityCopy.Subject is empty")
	}
	if cardinalityCopy.NoResults == "" {
		t.Error("cardinalityCopy.NoResults is empty")
	}
	if labelPatternCopy.Subject == "" {
		t.Error("labelPatternCopy.Subject is empty")
	}
	if labelPatternCopy.NoResults == "" {
		t.Error("labelPatternCopy.NoResults is empty")
	}
}

func TestRenderEmpty_Terminal_NoResults(t *testing.T) {
	var buf bytes.Buffer
	cfg := &config.Config{Output: "terminal", NoColor: true}
	err := renderEmpty(cfg, &buf, cardinalityCopy, findings.SeverityMedium, 0, nil, 0)
	if err != nil {
		t.Fatalf("renderEmpty: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "No high-cardinality metrics") {
		t.Errorf("expected NoResults copy, got:\n%s", out)
	}
	if !strings.Contains(out, "remetric doctor") {
		t.Errorf("expected doctor hint, got:\n%s", out)
	}
}

func TestRenderEmpty_Terminal_AllFiltered(t *testing.T) {
	var buf bytes.Buffer
	cfg := &config.Config{Output: "terminal", NoColor: true}
	bySev := map[findings.Severity]int{
		findings.SeverityLow:    3,
		findings.SeverityMedium: 2,
	}
	err := renderEmpty(cfg, &buf, cardinalityCopy, findings.SeverityHigh, 5, bySev, 0)
	if err != nil {
		t.Fatalf("renderEmpty: %v", err)
	}
	out := buf.String()
	for _, want := range []string{
		"Filtered 5",
		"cardinality offenders",
		"3 low",
		"2 medium",
		"--min-severity low",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("expected %q in output:\n%s", want, out)
		}
	}
}

func TestRenderEmpty_Terminal_LabelPatternSubject(t *testing.T) {
	var buf bytes.Buffer
	cfg := &config.Config{Output: "terminal", NoColor: true}
	bySev := map[findings.Severity]int{findings.SeverityLow: 1}
	err := renderEmpty(cfg, &buf, labelPatternCopy, findings.SeverityMedium, 1, bySev, 0)
	if err != nil {
		t.Fatalf("renderEmpty: %v", err)
	}
	if !strings.Contains(buf.String(), "suspicious labels") {
		t.Errorf("expected label-pattern subject, got:\n%s", buf.String())
	}
}

func TestRenderEmpty_JSON_NoResults(t *testing.T) {
	var buf bytes.Buffer
	cfg := &config.Config{Output: "json"}
	err := renderEmpty(cfg, &buf, cardinalityCopy, findings.SeverityMedium, 0, nil, 0)
	if err != nil {
		t.Fatalf("renderEmpty: %v", err)
	}
	if !strings.Contains(buf.String(), `"findings": []`) {
		t.Errorf("expected empty findings envelope, got:\n%s", buf.String())
	}
}

func TestRenderEmpty_JSON_AllFiltered(t *testing.T) {
	var buf bytes.Buffer
	cfg := &config.Config{Output: "json"}
	bySev := map[findings.Severity]int{findings.SeverityLow: 3}
	err := renderEmpty(cfg, &buf, cardinalityCopy, findings.SeverityHigh, 3, bySev, 0)
	if err != nil {
		t.Fatalf("renderEmpty: %v", err)
	}
	if !strings.Contains(buf.String(), `"findings": []`) {
		t.Errorf("expected empty findings envelope, got:\n%s", buf.String())
	}
}

func TestRenderEmpty_Terminal_AppendsIgnoredFooter(t *testing.T) {
	var buf bytes.Buffer
	cfg := &config.Config{Output: "terminal", NoColor: true}
	err := renderEmpty(cfg, &buf, cardinalityCopy, findings.SeverityMedium, 0, nil, 2)
	if err != nil {
		t.Fatalf("renderEmpty: %v", err)
	}
	if !strings.Contains(buf.String(), "ignored: 2 findings") {
		t.Errorf("expected ignored footer in terminal empty output, got:\n%s", buf.String())
	}
}

func TestRenderEmpty_JSON_EmitsIgnoredCount(t *testing.T) {
	var buf bytes.Buffer
	cfg := &config.Config{Output: "json"}
	err := renderEmpty(cfg, &buf, cardinalityCopy, findings.SeverityMedium, 0, nil, 4)
	if err != nil {
		t.Fatalf("renderEmpty: %v", err)
	}
	if !strings.Contains(buf.String(), `"ignored_count": 4`) {
		t.Errorf("expected ignored_count in JSON empty envelope, got:\n%s", buf.String())
	}
}
