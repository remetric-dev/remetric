// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package ignore_test

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/remetric-dev/remetric/internal/findings"
	"github.com/remetric-dev/remetric/internal/ignore"
)

func TestNew_EmptyPatternsReturnsPassThroughFilter(t *testing.T) {
	f, err := ignore.New(ignore.Patterns{})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if f == nil {
		t.Fatal("New returned nil filter")
	}
	in := []findings.Finding{{ID: "a", Metric: "anything"}, {ID: "b", Alert: "Anything"}}
	kept, ignored := f.Apply(in)
	if ignored != 0 {
		t.Errorf("ignored = %d, want 0", ignored)
	}
	if diff := cmp.Diff(in, kept); diff != "" {
		t.Errorf("kept mismatch (-want +got):\n%s", diff)
	}
}

func TestNew_BadRegexReturnsError(t *testing.T) {
	_, err := ignore.New(ignore.Patterns{Metric: []string{"foo["}})
	if err == nil {
		t.Fatal("New(Patterns{Metric: bad}) returned nil err")
	}
	if !strings.Contains(err.Error(), "foo[") {
		t.Errorf("err = %v; want it to mention the bad pattern", err)
	}
}

func TestNew_EmptyPatternStringSkipped(t *testing.T) {
	f, err := ignore.New(ignore.Patterns{Metric: []string{"", "  ", "foo"}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	in := []findings.Finding{{ID: "1", Metric: "foo"}, {ID: "2", Metric: ""}, {ID: "3", Metric: "bar"}}
	_, ignored := f.Apply(in)
	if ignored != 1 {
		t.Errorf("ignored = %d, want 1 (only id=1 matches 'foo')", ignored)
	}
}

func TestFilter_Apply_MetricPatternDrops(t *testing.T) {
	f, err := ignore.New(ignore.Patterns{Metric: []string{"node_.*"}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	in := []findings.Finding{
		{ID: "1", Metric: "node_cpu_seconds_total"},
		{ID: "2", Metric: "kafka_messages_total"},
	}
	kept, ignored := f.Apply(in)
	if ignored != 1 {
		t.Errorf("ignored = %d, want 1", ignored)
	}
	if len(kept) != 1 || kept[0].ID != "2" {
		t.Errorf("kept = %v, want only id=2", kept)
	}
}

func TestFilter_Apply_LabelPatternDrops(t *testing.T) {
	f, err := ignore.New(ignore.Patterns{Label: []string{"user_id"}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	in := []findings.Finding{
		{ID: "1", Evidence: findings.Evidence{Label: "user_id"}},
		{ID: "2", Evidence: findings.Evidence{Label: "region"}},
	}
	kept, ignored := f.Apply(in)
	if ignored != 1 {
		t.Errorf("ignored = %d, want 1", ignored)
	}
	if len(kept) != 1 || kept[0].ID != "2" {
		t.Errorf("kept = %v, want only id=2", kept)
	}
}

func TestFilter_Apply_AlertPatternDrops(t *testing.T) {
	f, err := ignore.New(ignore.Patterns{Alert: []string{"HighMemoryUsage"}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	in := []findings.Finding{
		{ID: "1", Alert: "HighMemoryUsage"},
		{ID: "2", Alert: "DiskFull"},
	}
	kept, ignored := f.Apply(in)
	if ignored != 1 {
		t.Errorf("ignored = %d, want 1", ignored)
	}
	if len(kept) != 1 || kept[0].ID != "2" {
		t.Errorf("kept = %v, want only id=2", kept)
	}
}

func TestFilter_Apply_AnchoredFullMatchNotSubstring(t *testing.T) {
	f, err := ignore.New(ignore.Patterns{Metric: []string{"go"}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	in := []findings.Finding{
		{ID: "1", Metric: "go"},
		{ID: "2", Metric: "mongo_total"},
		{ID: "3", Metric: "go_routines"},
	}
	_, ignored := f.Apply(in)
	if ignored != 1 {
		t.Errorf("ignored = %d, want 1 (only id=1 is exact match for 'go')", ignored)
	}
}

func TestFilter_Apply_EmptyFieldNotMatched(t *testing.T) {
	// --ignore-alert='.*' MUST NOT drop a non-alert finding (.Alert == "").
	f, err := ignore.New(ignore.Patterns{Alert: []string{".*"}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	in := []findings.Finding{
		{ID: "1", Metric: "foo"}, // .Alert == ""
		{ID: "2", Alert: "X"},
	}
	_, ignored := f.Apply(in)
	if ignored != 1 {
		t.Errorf("ignored = %d, want 1 (only id=2 has .Alert set)", ignored)
	}
}

func TestFilter_Apply_OrSemanticsAcrossFields(t *testing.T) {
	// Cardinality findings carry BOTH .Metric and Evidence.Label. A drop
	// triggered by EITHER side counts.
	f, err := ignore.New(ignore.Patterns{Label: []string{"pod"}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	in := []findings.Finding{
		{ID: "1", Metric: "kafka_total", Evidence: findings.Evidence{Label: "pod"}},
	}
	_, ignored := f.Apply(in)
	if ignored != 1 {
		t.Errorf("ignored = %d, want 1 (Label match drops even though Metric doesn't)", ignored)
	}
}

func TestFilter_Apply_MultiplePatternsOredWithinField(t *testing.T) {
	f, err := ignore.New(ignore.Patterns{Metric: []string{"node_.*", "go_.*"}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	in := []findings.Finding{
		{ID: "1", Metric: "node_cpu"},
		{ID: "2", Metric: "go_routines"},
		{ID: "3", Metric: "kafka_total"},
	}
	_, ignored := f.Apply(in)
	if ignored != 2 {
		t.Errorf("ignored = %d, want 2 (any of node_.*, go_.* drops)", ignored)
	}
}

func TestFilter_Apply_PreservesOrder(t *testing.T) {
	f, err := ignore.New(ignore.Patterns{Metric: []string{"drop_.*"}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	in := []findings.Finding{
		{ID: "1", Metric: "a"},
		{ID: "2", Metric: "drop_x"},
		{ID: "3", Metric: "b"},
		{ID: "4", Metric: "drop_y"},
		{ID: "5", Metric: "c"},
	}
	kept, ignored := f.Apply(in)
	if ignored != 2 {
		t.Fatalf("ignored = %d, want 2", ignored)
	}
	want := []string{"1", "3", "5"}
	got := []string{}
	for _, f := range kept {
		got = append(got, f.ID)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("kept order mismatch (-want +got):\n%s", diff)
	}
}
