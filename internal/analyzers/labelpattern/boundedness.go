// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package labelpattern

import (
	"unicode"

	"github.com/remetric-dev/remetric/internal/findings"
)

// looksUnbounded reports whether sampled label values exhibit the
// shape of unbounded identifiers (UUIDs, hashes, paths) rather than a
// bounded enum (status codes, small ints).
//
// Returns true (looks unbounded) when ANY of:
//   - empty input (no evidence to contradict the name match)
//   - mean length > 8
//   - max length >= 2 * min length (varied lengths)
//   - the sample contains both non-digits AND a separator (`-`, `_`, `/`, `.`)
//
// Returns false (looks bounded) when all samples are short, similar
// length, and digit-dominated — the canonical small-enum shape.
func looksUnbounded(values []string) bool {
	if len(values) == 0 {
		return true
	}
	st := scanShape(values)
	meanLen := st.sum / len(values)
	switch {
	case meanLen > 8:
		return true
	case st.minLen > 0 && st.maxLen >= 2*st.minLen:
		return true
	case st.hasNonDigit && st.hasSeparator:
		return true
	default:
		return false
	}
}

// shape captures the aggregate character statistics looksUnbounded
// uses to classify a sample.
type shape struct {
	sum, minLen, maxLen       int
	hasNonDigit, hasSeparator bool
}

// scanShape walks every rune of every value once and accumulates the
// stats looksUnbounded needs. Extracted so the caller stays a flat
// classification ladder.
func scanShape(values []string) shape {
	st := shape{minLen: -1}
	for _, v := range values {
		n := len(v)
		st.sum += n
		if st.minLen == -1 || n < st.minLen {
			st.minLen = n
		}
		if n > st.maxLen {
			st.maxLen = n
		}
		for _, r := range v {
			if !unicode.IsDigit(r) {
				st.hasNonDigit = true
			}
			if isSeparator(r) {
				st.hasSeparator = true
			}
		}
	}
	return st
}

// isSeparator reports whether r is one of the punctuation characters
// commonly found inside identifier-shaped label values.
func isSeparator(r rune) bool {
	return r == '-' || r == '_' || r == '/' || r == '.'
}

// downgradeOnce shifts the severity one tier lower, clamping at Low.
// Used when a labelpattern finding's sampled values look bounded
// (likely false positive from a name-only match).
func downgradeOnce(s findings.Severity) findings.Severity {
	if s <= findings.SeverityLow {
		return findings.SeverityLow
	}
	return s - 1
}
