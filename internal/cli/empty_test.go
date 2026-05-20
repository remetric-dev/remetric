// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package cli

import (
	"testing"

	"github.com/google/go-cmp/cmp"

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
