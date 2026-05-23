// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

// Package findings defines the shared data types for cardinality
// and other analyzer outputs.
package findings

import (
	"encoding/json"
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

// lower returns the canonical lower-case form of s, used as JSON map
// keys and `MarshalJSON` output.
func (s Severity) lower() string { return strings.ToLower(s.String()) }

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

// MarshalJSON encodes the severity as a lower-case string
// (critical|high|medium|low) for the public JSON wire form.
func (s Severity) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.lower())
}

// UnmarshalJSON accepts a case-insensitive severity name.
// A JSON null is a no-op, per the encoding/json convention.
func (s *Severity) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		return nil
	}
	var str string
	if err := json.Unmarshal(b, &str); err != nil {
		return fmt.Errorf("severity: %w", err)
	}
	sev, err := ParseSeverity(str)
	if err != nil {
		return err
	}
	*s = sev
	return nil
}
