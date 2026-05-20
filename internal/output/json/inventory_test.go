// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package json

import (
	"encoding/json"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestLabelInventory_JSONShape(t *testing.T) {
	inv := LabelInventory{
		Metric: "http_requests_total",
		Labels: []LabelStat{
			{Name: "user_id", UniqueValues: 5000, SampleValues: []string{"u-1", "u-2"}},
			{Name: "status", UniqueValues: 5},
		},
	}
	b, err := json.Marshal(inv)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	want := map[string]any{
		"metric": "http_requests_total",
		"labels": []any{
			map[string]any{
				"name":          "user_id",
				"unique_values": float64(5000),
				"sample_values": []any{"u-1", "u-2"},
			},
			map[string]any{
				"name":          "status",
				"unique_values": float64(5),
			},
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("inventory JSON mismatch (-want +got):\n%s", diff)
	}
}
