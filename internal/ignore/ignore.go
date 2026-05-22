// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

// Package ignore implements post-render filtering of findings by
// regex patterns matched against structured fields (Metric, Label, Alert).
//
// Patterns are anchored full-match: the user writes `foo_.*` and the
// filter wraps it as `^(foo_.*)$`. Empty / whitespace-only patterns are
// silently skipped. A zero-value Filter passes every finding through.
package ignore

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/remetric-dev/remetric/internal/findings"
)

// Patterns groups raw regex strings by the field they match.
type Patterns struct {
	Metric []string
	Label  []string
	Alert  []string
}

// Filter applies compiled regex patterns to a slice of findings.
// The zero value passes every finding through.
type Filter struct {
	metric []*regexp.Regexp
	label  []*regexp.Regexp
	alert  []*regexp.Regexp
}

// New compiles Patterns into a Filter. Returns an error on any
// uncompilable regex. Empty / whitespace-only patterns are silently
// dropped (common when env-variable lists are blank).
func New(p Patterns) (*Filter, error) {
	mc, err := compile("metric", p.Metric)
	if err != nil {
		return nil, err
	}
	lc, err := compile("label", p.Label)
	if err != nil {
		return nil, err
	}
	ac, err := compile("alert", p.Alert)
	if err != nil {
		return nil, err
	}
	return &Filter{metric: mc, label: lc, alert: ac}, nil
}

func compile(kind string, raw []string) ([]*regexp.Regexp, error) {
	out := make([]*regexp.Regexp, 0, len(raw))
	for _, s := range raw {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		re, err := regexp.Compile("^(" + s + ")$")
		if err != nil {
			return nil, fmt.Errorf("bad %s pattern %q: %w", kind, s, err)
		}
		out = append(out, re)
	}
	return out, nil
}

// Apply returns the kept findings and the number dropped. Input order
// is preserved among kept findings. When the filter has no patterns at
// all, Apply returns the input slice unchanged (no allocation).
func (f *Filter) Apply(in []findings.Finding) (kept []findings.Finding, ignored int) {
	if f == nil || (len(f.metric) == 0 && len(f.label) == 0 && len(f.alert) == 0) {
		return in, 0
	}
	kept = make([]findings.Finding, 0, len(in))
	for _, fnd := range in {
		if f.drops(fnd) {
			ignored++
			continue
		}
		kept = append(kept, fnd)
	}
	return kept, ignored
}

// drops reports whether any of the configured pattern lists matches one
// of the structured fields of fnd. Order is metric -> label -> alert,
// short-circuit on first match.
func (f *Filter) drops(fnd findings.Finding) bool {
	if fnd.Metric != "" && matchAny(f.metric, fnd.Metric) {
		return true
	}
	if fnd.Evidence.Label != "" && matchAny(f.label, fnd.Evidence.Label) {
		return true
	}
	if fnd.Alert != "" && matchAny(f.alert, fnd.Alert) {
		return true
	}
	return false
}

func matchAny(rs []*regexp.Regexp, s string) bool {
	for _, r := range rs {
		if r.MatchString(s) {
			return true
		}
	}
	return false
}
