// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package prometheus_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	prom "github.com/remetric-dev/remetric/internal/prometheus"
)

func TestSamplePair_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want prom.SamplePair
	}{
		{
			name: "integer ts and string value",
			in:   `[1715000000, "1"]`,
			want: prom.SamplePair{Timestamp: time.UnixMilli(1715000000000).UTC(), Value: 1},
		},
		{
			name: "float ts and float string value",
			in:   `[1715000000.500, "0.5"]`,
			want: prom.SamplePair{Timestamp: time.UnixMilli(1715000000500).UTC(), Value: 0.5},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got prom.SamplePair
			if err := json.Unmarshal([]byte(tt.in), &got); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("SamplePair mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSamplePair_UnmarshalJSON_Errors(t *testing.T) {
	tests := []struct{ name, in string }{
		{"empty array", `[]`},
		{"missing value", `[1715000000]`},
		{"non-numeric ts", `["abc", "1"]`},
		{"non-string value", `[1715000000, 1]`},
		{"unparseable value", `[1715000000, "not-a-number"]`},
		{"too many elements", `[1715000000, "1", 3]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got prom.SamplePair
			if err := json.Unmarshal([]byte(tt.in), &got); err == nil {
				t.Errorf("Unmarshal(%q) = nil, want error", tt.in)
			}
		})
	}
}
