// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package unusedmetrics

import (
	"strings"
	"testing"
)

func TestRenderFix(t *testing.T) {
	out, err := RenderFix("etcd_request_duration_seconds_bucket")
	if err != nil {
		t.Fatalf("RenderFix: %v", err)
	}
	for _, want := range []string{
		"metric_relabel_configs:",
		"source_labels: [__name__]",
		"regex: 'etcd_request_duration_seconds_bucket'",
		"action: drop",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
}
