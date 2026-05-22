// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package dashboardhygiene

import (
	"strings"
	"testing"
)

func TestBuildFix_WithURLAndPanelList(t *testing.T) {
	got := buildFix(
		"Frontend SLOs",
		"node_disk_io_now",
		"https://grafana.example.com/d/abc123/frontend-slos",
		[]string{"Disk I/O - last 5m", "Disk I/O - last 1h"},
	)
	for _, want := range []string{
		`Edit dashboard "Frontend SLOs"`,
		`https://grafana.example.com/d/abc123/frontend-slos`,
		`Restore metric "node_disk_io_now"`,
		`Disk I/O - last 5m`,
		`Disk I/O - last 1h`,
		`https://remetric.dev/findings/broken-panel`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("buildFix output missing %q\nfull output:\n%s", want, got)
		}
	}
}

func TestBuildFix_OmitsURLWhenEmpty(t *testing.T) {
	got := buildFix("D", "m", "", []string{"P"})
	if strings.Contains(got, "(http") {
		t.Errorf("empty URL should drop the URL parenthetical; got:\n%s", got)
	}
	if strings.Contains(got, "https://grafana") {
		t.Errorf("empty URL must not emit any grafana URL; got:\n%s", got)
	}
	if !strings.Contains(got, `Edit dashboard "D"`) {
		t.Errorf("title still expected; got:\n%s", got)
	}
}

func TestBuildFix_TruncatesPanelListPast10(t *testing.T) {
	titles := []string{
		"P01", "P02", "P03", "P04", "P05",
		"P06", "P07", "P08", "P09", "P10",
		"P11", "P12",
	}
	got := buildFix("D", "m", "", titles)
	if !strings.Contains(got, "P10") {
		t.Errorf("10th panel must still appear; got:\n%s", got)
	}
	if strings.Contains(got, "P11") {
		t.Errorf("11th panel must be elided; got:\n%s", got)
	}
	if !strings.Contains(got, "... and 2 more") {
		t.Errorf("expected '... and 2 more' tail; got:\n%s", got)
	}
}
