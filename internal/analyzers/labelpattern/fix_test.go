// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package labelpattern

import (
	"strings"
	"testing"
)

func TestRenderFix(t *testing.T) {
	out, err := RenderFix("user_id", []string{"http_requests_total", "istio_requests_total"})
	if err != nil {
		t.Fatalf("RenderFix: %v", err)
	}

	for _, want := range []string{
		"metric_relabel_configs:",
		"regex: 'user_id'",
		"action: labeldrop",
		"# Affected metrics (2):",
		"#   - http_requests_total",
		"#   - istio_requests_total",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("RenderFix output missing %q\nGot:\n%s", want, out)
		}
	}
}

func TestRenderFix_NoMetrics(t *testing.T) {
	out, err := RenderFix("user_id", nil)
	if err != nil {
		t.Fatalf("RenderFix: %v", err)
	}
	if !strings.Contains(out, "# Affected metrics (0):") {
		t.Errorf("expected '# Affected metrics (0):' in output:\n%s", out)
	}
}
