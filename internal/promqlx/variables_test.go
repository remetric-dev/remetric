// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package promqlx

import "testing"

func TestSanitiseGrafanaVars(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"none", `rate(http_requests_total[5m])`, `rate(http_requests_total[5m])`},
		{"dollar_var", `rate(http_requests_total{job="$service"}[5m])`, `rate(http_requests_total{job="__remetric_var__"}[5m])`},
		{"curly_var", `rate(http_requests_total{job="${service}"}[5m])`, `rate(http_requests_total{job="__remetric_var__"}[5m])`},
		{"multi_var", `up{job="$service",instance="${host}"}`, `up{job="__remetric_var__",instance="__remetric_var__"}`},
		{"with_special_chars", `up{job="${service:raw}"}`, `up{job="__remetric_var__"}`},
		{"metric_name_var", `$metric_name{job="api"}`, `__remetric_var__{job="api"}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := sanitiseGrafanaVars(c.in)
			if got != c.want {
				t.Errorf("sanitiseGrafanaVars(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestIsSentinel(t *testing.T) {
	if !isSentinel("__remetric_var__") {
		t.Error("isSentinel(sentinel) = false, want true")
	}
	if isSentinel("__remetric_var") {
		t.Error("isSentinel(non-sentinel) = true, want false")
	}
	// Concatenation forms: sanitiser leaves the sentinel as a substring
	// when a template variable is glued to literal text in the source.
	if !isSentinel("__remetric_var___total") {
		t.Error("isSentinel(suffix-concat) = false, want true")
	}
	if !isSentinel("foo___remetric_var__") {
		t.Error("isSentinel(prefix-concat) = false, want true")
	}
}
