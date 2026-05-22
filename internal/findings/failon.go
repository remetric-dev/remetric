// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package findings

import "strings"

// ParseFailOn parses the --fail-on flag value. Returns the parsed severity,
// whether the gate is enabled (false iff value is empty or "none"), and any
// parse error. Case-insensitive; whitespace-tolerant.
func ParseFailOn(v string) (Severity, bool, error) {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "", "none":
		return 0, false, nil
	}
	sev, err := ParseSeverity(v)
	if err != nil {
		return 0, false, err
	}
	return sev, true, nil
}

// ShouldFail returns true when fs contains at least one finding whose
// Severity is at or above threshold. When enabled is false (i.e. the user
// passed --fail-on=none or omitted the flag), always returns false.
func ShouldFail(threshold Severity, enabled bool, fs []Finding) bool {
	if !enabled {
		return false
	}
	for _, f := range fs {
		if f.Severity >= threshold {
			return true
		}
	}
	return false
}
