// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package promqlx

import (
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func TestExtractFromQuery(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want []string
	}{
		{"bare_name", `up`, []string{"up"}},
		{"rate", `rate(http_requests_total[5m])`, []string{"http_requests_total"}},
		{"matcher", `http_requests_total{job="api"}`, []string{"http_requests_total"}},
		{"by_label", `sum by (job) (rate(http_requests_total[5m]))`, []string{"http_requests_total"}},
		{
			name: "binary_op",
			in:   `node_filesystem_avail_bytes / node_filesystem_size_bytes`,
			want: []string{"node_filesystem_avail_bytes", "node_filesystem_size_bytes"},
		},
		{
			name: "subquery",
			in:   `max_over_time(rate(http_requests_total[5m])[1h:1m])`,
			want: []string{"http_requests_total"},
		},
		{
			name: "label_replace",
			in:   `label_replace(up, "host", "$1", "instance", "(.+):.+")`,
			want: []string{"up"},
		},
		{
			name: "join",
			in:   `up * on(instance) group_left(role) node_metadata`,
			want: []string{"node_metadata", "up"},
		},
		{
			name: "grafana_var",
			in:   `rate(http_requests_total{job="$service"}[5m])`,
			want: []string{"http_requests_total"},
		},
		{
			name: "grafana_var_in_metric_name",
			in:   `$metric_name{job="api"}`,
			want: []string{}, // sentinel filtered, no real metric names captured
		},
		{
			name: "grafana_var_suffix_concat",
			in:   `${metric_name}_total`,
			want: []string{}, // sanitises to __remetric_var___total - must be filtered as sentinel artifact
		},
		{
			name: "grafana_var_prefix_concat",
			in:   `foo_${suffix}`,
			want: []string{}, // sanitises to foo___remetric_var__ - must be filtered as sentinel artifact
		},
		{
			name: "deduplication",
			in:   `up + up + up`,
			want: []string{"up"},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ExtractFromQuery(c.in)
			if err != nil {
				t.Fatalf("ExtractFromQuery(%q) error: %v", c.in, err)
			}
			gotSorted := setToSortedSlice(got)
			wantSorted := append([]string(nil), c.want...)
			sort.Strings(wantSorted)
			if diff := cmp.Diff(wantSorted, gotSorted, cmpopts.EquateEmpty()); diff != "" {
				t.Errorf("ExtractFromQuery(%q) mismatch (-want +got):\n%s", c.in, diff)
			}
		})
	}
}

func TestExtractFromQuery_ParseError(t *testing.T) {
	_, err := ExtractFromQuery(`not (a valid expression`)
	if err == nil {
		t.Error("ExtractFromQuery(invalid) = nil error, want error")
	}
}

func TestExtractFromQuery_Empty(t *testing.T) {
	got, err := ExtractFromQuery("")
	if err != nil {
		t.Fatalf("ExtractFromQuery(empty) error: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("ExtractFromQuery(empty) = %v, want empty", got)
	}
}

func setToSortedSlice(s map[string]struct{}) []string {
	out := make([]string, 0, len(s))
	for k := range s {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
