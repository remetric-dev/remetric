package cardinality

import (
	"testing"

	"github.com/remetric-dev/remetric/internal/findings"
)

func TestSeverity_TableDriven(t *testing.T) {
	const total = 1_000_000
	tests := []struct {
		name        string
		seriesCount int64
		want        findings.Severity
	}{
		{"explicit critical absolute", 350_000, findings.SeverityCritical},
		{"critical percentage", 160_000, findings.SeverityCritical}, // 16%
		{"high absolute", 120_000, findings.SeverityHigh},
		{"high percentage", 60_000, findings.SeverityHigh}, // 6%
		{"medium", 30_000, findings.SeverityMedium},
		{"low fallback", 10_000, findings.SeverityLow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := severity(tt.seriesCount, total)
			if got != tt.want {
				t.Errorf("severity(%d, %d) = %v, want %v", tt.seriesCount, total, got, tt.want)
			}
		})
	}
}
