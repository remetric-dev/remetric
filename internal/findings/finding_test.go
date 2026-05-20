// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package findings

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestFinding_ZeroValueIsUsable(t *testing.T) {
	var f Finding
	if f.Severity != SeverityLow {
		t.Errorf("zero Finding.Severity = %v, want SeverityLow", f.Severity)
	}
	if f.Category != "" {
		t.Errorf("zero Finding.Category = %q, want empty", f.Category)
	}
}

func TestCategory_String(t *testing.T) {
	if string(CategoryCardinality) != "cardinality" {
		t.Errorf("CategoryCardinality = %q, want %q", CategoryCardinality, "cardinality")
	}
}

func TestFinding_MarshalJSON(t *testing.T) {
	f := Finding{
		ID:       "card-foo-bar",
		Severity: SeverityHigh,
		Category: CategoryCardinality,
		Title:    "high cardinality in foo",
		Metric:   "foo",
		Evidence: Evidence{
			Label:        "bar",
			UniqueValues: 1234,
			SampleValues: []string{"a", "b"},
			SeriesCount:  98765,
			Description:  "bar has 1234 unique values inside foo",
		},
		Fix: Fix{
			Type:   "scrape_config",
			Config: "drop:\n  bar\n",
		},
		Impact: Impact{
			SeriesReduction:  50000,
			Percentage:       50.5,
			EstimationMethod: "labeldrop_upper_bound",
		},
	}

	b, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}

	want := map[string]any{
		"id":       "card-foo-bar",
		"severity": "high",
		"category": "cardinality",
		"title":    "high cardinality in foo",
		"metric":   "foo",
		"evidence": map[string]any{
			"label":         "bar",
			"unique_values": float64(1234),
			"sample_values": []any{"a", "b"},
			"series_count":  float64(98765),
			"description":   "bar has 1234 unique values inside foo",
		},
		"fix": map[string]any{
			"type":   "scrape_config",
			"config": "drop:\n  bar\n",
		},
		"impact": map[string]any{
			"series_reduction":  float64(50000),
			"percentage":        50.5,
			"estimation_method": "labeldrop_upper_bound",
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Finding JSON mismatch (-want +got):\n%s", diff)
	}
}

func TestFinding_OmitsEmpty(t *testing.T) {
	f := Finding{
		ID:       "min-001",
		Severity: SeverityLow,
		Category: CategoryLabelPatterns,
		Title:    "minimal",
		Evidence: Evidence{Description: "x"},
	}
	b, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	s := string(b)
	for _, banned := range []string{`"metric"`, `"documentation_url"`, `"label"`, `"unique_values"`, `"sample_values"`, `"series_count"`} {
		if strings.Contains(s, banned) {
			t.Errorf("Marshal output unexpectedly contains %s in: %s", banned, s)
		}
	}
}
