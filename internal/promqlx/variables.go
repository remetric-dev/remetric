// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

// Package promqlx wraps the official Prometheus PromQL parser to
// extract the set of metric names referenced in expressions.
// It handles Grafana template variables ($var, ${var}, ${var:format})
// by substituting them with a sentinel identifier before parsing.
package promqlx

import (
	"regexp"
	"strings"
)

// sentinel replaces Grafana template variables that would otherwise
// break PromQL parsing. The string is a valid PromQL identifier
// (matches `[a-zA-Z_][a-zA-Z0-9_]*`) and is filtered out of the
// final metric-name set.
const sentinel = "__remetric_var__"

// grafanaVarPattern matches Grafana template variables in three forms:
//
//	$name                    - bare-dollar form
//	${name}                  - braced form
//	${name:format}           - braced form with format modifier
var grafanaVarPattern = regexp.MustCompile(`\$\{[^}]+\}|\$[a-zA-Z_][a-zA-Z0-9_]*`)

// sanitiseGrafanaVars replaces every Grafana template variable in
// expr with the sentinel identifier. Result is valid PromQL syntax
// (assuming the rest of the expression was valid).
func sanitiseGrafanaVars(expr string) string {
	return grafanaVarPattern.ReplaceAllString(expr, sentinel)
}

// isSentinel reports whether name is, or was derived from, the
// sentinel identifier used to stand in for Grafana template variables.
//
// A substring (not equality) check is required because Grafana panels
// often concatenate template variables with literal text, e.g.
// "${metric}_total" or "foo_${suffix}". After sanitisation these
// become "__remetric_var___total" / "foo___remetric_var__", which are
// valid PromQL identifiers and parse cleanly. Any name containing the
// sentinel is therefore a sanitiser artifact, not a real metric, and
// must be filtered out of the extracted set.
func isSentinel(name string) bool {
	return strings.Contains(name, sentinel)
}
