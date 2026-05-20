// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package promqlx

import (
	"fmt"
	"strings"

	"github.com/prometheus/prometheus/promql/parser"
)

// ExtractFromQuery parses a PromQL expression and returns the set
// of metric names it references. Grafana template variables ($var,
// ${var}) are sanitised before parsing; the sentinel placeholder
// is filtered out of the returned set.
//
// An empty input returns an empty set without parsing.
func ExtractFromQuery(expr string) (map[string]struct{}, error) {
	if strings.TrimSpace(expr) == "" {
		return map[string]struct{}{}, nil
	}
	sanitised := sanitiseGrafanaVars(expr)
	tree, err := parser.ParseExpr(sanitised)
	if err != nil {
		return nil, fmt.Errorf("promqlx: parse %q: %w", expr, err)
	}
	out := map[string]struct{}{}
	parser.Inspect(tree, func(node parser.Node, _ []parser.Node) error {
		vs, ok := node.(*parser.VectorSelector)
		if !ok {
			return nil
		}
		name := vs.Name
		// Some PromQL constructs put the metric name in the matchers,
		// e.g. {__name__="up"}. Honour that too.
		if name == "" {
			for _, m := range vs.LabelMatchers {
				if m.Name == "__name__" {
					name = m.Value
					break
				}
			}
		}
		if name == "" || isSentinel(name) {
			return nil
		}
		out[name] = struct{}{}
		return nil
	})
	return out, nil
}
