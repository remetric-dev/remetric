// Package findings defines the shared data types for cardinality
// and other analyzer outputs.
package findings

import (
	"fmt"
	"strings"
)

// Severity is the impact ranking attached to a Finding.
type Severity int

// Severity levels, ordered from least to most impactful.
const (
	SeverityLow Severity = iota
	SeverityMedium
	SeverityHigh
	SeverityCritical
)

// String returns the canonical upper-case label for s.
func (s Severity) String() string {
	switch s {
	case SeverityLow:
		return "LOW"
	case SeverityMedium:
		return "MEDIUM"
	case SeverityHigh:
		return "HIGH"
	case SeverityCritical:
		return "CRITICAL"
	default:
		return "UNKNOWN"
	}
}

// ParseSeverity parses a case-insensitive severity name.
func ParseSeverity(s string) (Severity, error) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "low":
		return SeverityLow, nil
	case "medium":
		return SeverityMedium, nil
	case "high":
		return SeverityHigh, nil
	case "critical":
		return SeverityCritical, nil
	}
	return 0, fmt.Errorf("unknown severity %q", s)
}
