// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package cardinality

import (
	"strings"
	"testing"
)

func TestRenderFix(t *testing.T) {
	got, err := renderFix("istio_requests_total", "destination_principal")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	for _, want := range []string{
		"metric_relabel_configs:",
		`regex: "istio_requests_total"`,
		`regex: "destination_principal"`,
		"action: keep",
		"action: labeldrop",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("rendered fix missing %q\ngot:\n%s", want, got)
		}
	}
}

func TestRenderFix_EscapesSpecialChars(t *testing.T) {
	got, err := renderFix("metric.with.dots", "label_id")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(got, `regex: "metric.with.dots"`) {
		t.Errorf("expected verbatim metric name in regex; got:\n%s", got)
	}
}
