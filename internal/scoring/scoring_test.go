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
