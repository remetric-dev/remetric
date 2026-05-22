// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

// Package labelpattern flags high-cardinality labels whose names match
// well-known unbounded-identifier patterns (uuid, *_id, path, etc.).
package labelpattern

import "regexp"

// defaultPatternSources are the verbatim regex patterns from the spec
// (§6.3). Compiled lazily via DefaultPatterns.
var defaultPatternSources = []string{
	`(?i).*(uuid|guid).*`,
	`(?i).*_id$`,
	`(?i)^(path|url|uri|endpoint)$`,
	`(?i).*(trace|span)_id.*`,
	`(?i)^(session|request)_.*`,
	`(?i)^(email|user(name)?)$`,
}

// boundedLabels are operational labels that never get flagged regardless
// of name shape - they may have high cardinality by convention but
// dropping them would break dashboards. v0.1 hard-codes the list; a
// future phase will expose it via config.
var boundedLabels = map[string]bool{
	"cluster":     true,
	"environment": true,
	"region":      true,
	"namespace":   true,
	"job":         true,
	"instance":    true,
}

// DefaultPatterns returns the compiled default suspicious-label patterns.
// All patterns are case-insensitive. The returned slice is safe to share.
func DefaultPatterns() []*regexp.Regexp {
	out := make([]*regexp.Regexp, 0, len(defaultPatternSources))
	for _, src := range defaultPatternSources {
		out = append(out, regexp.MustCompile(src))
	}
	return out
}

// CompilePatterns compiles a user-supplied list of regex sources.
// Returns the first compile error if any source is invalid.
func CompilePatterns(sources []string) ([]*regexp.Regexp, error) {
	out := make([]*regexp.Regexp, 0, len(sources))
	for _, src := range sources {
		re, err := regexp.Compile(src)
		if err != nil {
			return nil, err
		}
		out = append(out, re)
	}
	return out, nil
}

// MatchesAny reports whether name matches any of the patterns.
func MatchesAny(patterns []*regexp.Regexp, name string) bool {
	for _, p := range patterns {
		if p.MatchString(name) {
			return true
		}
	}
	return false
}

// IsBounded reports whether name is on the bounded-label allowlist.
func IsBounded(name string) bool {
	return boundedLabels[name]
}
