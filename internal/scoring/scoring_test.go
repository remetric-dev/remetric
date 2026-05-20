// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package scoring

import (
	"testing"

	"github.com/remetric-dev/remetric/internal/findings"
)

func TestCardinalitySeverity(t *testing.T) {
	cases := []struct {
		name  string
		count int64
		total int64
		want  findings.Severity
	}{
		{"critical_absolute", 400_000, 10_000_000, findings.SeverityCritical},
		{"critical_percentage", 200_000, 1_000_000, findings.SeverityCritical}, // 20%
		{"high_absolute", 150_000, 10_000_000, findings.SeverityHigh},
		{"high_percentage", 60_000, 1_000_000, findings.SeverityHigh}, // 6%
		{"medium", 30_000, 10_000_000, findings.SeverityMedium},
		{"low", 5_000, 10_000_000, findings.SeverityLow},
		{"boundary_300k", 300_000, 10_000_000, findings.SeverityHigh}, // strict >
		{"boundary_100k", 100_000, 10_000_000, findings.SeverityMedium},
		{"boundary_25k", 25_000, 10_000_000, findings.SeverityLow},
		{"zero_total", 1_000, 0, findings.SeverityLow},
		{"negative_total", 1_000, -1, findings.SeverityLow},
		{"pct_exactly_15", 150_000, 1_000_000, findings.SeverityHigh}, // strict > 15 means 15.0 → not Critical
		{"pct_exactly_5", 50_000, 1_000_000, findings.SeverityMedium}, // strict > 5 means 5.0 → not High
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := CardinalitySeverity(c.count, c.total)
			if got != c.want {
				t.Errorf("CardinalitySeverity(%d, %d) = %v, want %v", c.count, c.total, got, c.want)
			}
		})
	}
}

func TestLabelPatternSeverity(t *testing.T) {
	cases := []struct {
		name string
		uniq int
		want findings.Severity
	}{
		{"critical_high_uniq", 10_000, findings.SeverityCritical},
		{"critical_boundary", 5001, findings.SeverityCritical},
		{"high", 2_500, findings.SeverityHigh},
		{"high_boundary", 1001, findings.SeverityHigh},
		{"medium", 500, findings.SeverityMedium},
		{"medium_boundary", 101, findings.SeverityMedium},
		{"low", 10, findings.SeverityLow},
		{"low_zero", 0, findings.SeverityLow},
		{"critical_at_5000", 5000, findings.SeverityHigh}, // strict >
		{"high_at_1000", 1000, findings.SeverityMedium},
		{"medium_at_100", 100, findings.SeverityLow},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := LabelPatternSeverity(c.uniq)
			if got != c.want {
				t.Errorf("LabelPatternSeverity(%d) = %v, want %v", c.uniq, got, c.want)
			}
		})
	}
}

func TestUnusedMetricSeverity(t *testing.T) {
	cases := []struct {
		name  string
		count int64
		want  findings.Severity
	}{
		{"critical", 200_000, findings.SeverityCritical},
		{"critical_boundary", 100_001, findings.SeverityCritical},
		{"high_at_100k", 100_000, findings.SeverityHigh},
		{"high", 60_000, findings.SeverityHigh},
		{"high_boundary", 25_001, findings.SeverityHigh},
		{"medium_at_25k", 25_000, findings.SeverityMedium},
		{"medium", 10_000, findings.SeverityMedium},
		{"medium_boundary", 5_001, findings.SeverityMedium},
		{"low_at_5k", 5_000, findings.SeverityLow},
		{"low", 100, findings.SeverityLow},
		{"low_zero", 0, findings.SeverityLow},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := UnusedMetricSeverity(c.count)
			if got != c.want {
				t.Errorf("UnusedMetricSeverity(%d) = %v, want %v", c.count, got, c.want)
			}
		})
	}
}
