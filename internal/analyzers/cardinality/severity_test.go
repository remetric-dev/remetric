package cardinality

import (
	"testing"

	"github.com/remetric-dev/remetric/internal/findings"
)

func TestSeverity_TableDriven(t *testing.T) {
	const defaultTotal = 1_000_000
	// boundaryTotal isolates the strict-greater-than absolute thresholds from
	// the percentage thresholds: at 10_000_000, neither 300k/100k/25k crosses
	// the pct gates, so the test exercises only the absolute boundary.
	const boundaryTotal = 10_000_000
	tests := []struct {
		name        string
		seriesCount int64
		total       int64 // zero → defaultTotal
		want        findings.Severity
	}{
		{"explicit critical absolute", 350_000, 0, findings.SeverityCritical},
		{"critical percentage", 160_000, 0, findings.SeverityCritical}, // 16%
		{"high absolute", 120_000, 0, findings.SeverityHigh},
		{"high percentage", 60_000, 0, findings.SeverityHigh}, // 6%
		{"medium", 30_000, 0, findings.SeverityMedium},
		{"low fallback", 10_000, 0, findings.SeverityLow},
		{"boundary 300k → High", 300_000, boundaryTotal, findings.SeverityHigh},
		{"boundary 100k → Medium", 100_000, boundaryTotal, findings.SeverityMedium},
		{"boundary 25k → Low", 25_000, boundaryTotal, findings.SeverityLow},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			total := tt.total
			if total == 0 {
				total = defaultTotal
			}
			got := severity(tt.seriesCount, total)
			if got != tt.want {
				t.Errorf("severity(%d, %d) = %v, want %v", tt.seriesCount, total, got, tt.want)
			}
		})
	}
}
