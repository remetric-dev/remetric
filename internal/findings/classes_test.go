// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package findings_test

import (
	"testing"

	"github.com/remetric-dev/remetric/internal/findings"
)

func TestFillDocURLs(t *testing.T) {
	tests := []struct {
		name string
		in   []findings.Finding
		want []findings.Finding
	}{
		{
			name: "populates empty DocURL from Class",
			in: []findings.Finding{
				{Class: findings.ClassHotLabel},
				{Class: findings.ClassUnusedMetric},
			},
			want: []findings.Finding{
				{Class: findings.ClassHotLabel, DocURL: "https://remetric.dev/findings/hot-label"},
				{Class: findings.ClassUnusedMetric, DocURL: "https://remetric.dev/findings/unused-metric"},
			},
		},
		{
			name: "preserves caller-provided DocURL",
			in: []findings.Finding{
				{Class: findings.ClassHotLabel, DocURL: "https://example.com/custom"},
			},
			want: []findings.Finding{
				{Class: findings.ClassHotLabel, DocURL: "https://example.com/custom"},
			},
		},
		{
			name: "leaves DocURL empty when Class is empty",
			in: []findings.Finding{
				{Class: ""},
			},
			want: []findings.Finding{
				{Class: ""},
			},
		},
		{
			name: "empty slice is a no-op",
			in:   nil,
			want: nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := append([]findings.Finding(nil), tt.in...)
			findings.FillDocURLs(got)
			if len(got) != len(tt.want) {
				t.Fatalf("len = %d, want %d", len(got), len(tt.want))
			}
			for i := range got {
				if got[i].DocURL != tt.want[i].DocURL {
					t.Errorf("[%d].DocURL = %q, want %q", i, got[i].DocURL, tt.want[i].DocURL)
				}
				if got[i].Class != tt.want[i].Class {
					t.Errorf("[%d].Class = %q, want %q", i, got[i].Class, tt.want[i].Class)
				}
			}
		})
	}
}

func TestDocURL(t *testing.T) {
	tests := []struct {
		name  string
		class findings.Class
		want  string
	}{
		{"hot-label", findings.ClassHotLabel, "https://remetric.dev/findings/hot-label"},
		{"unused-metric", findings.ClassUnusedMetric, "https://remetric.dev/findings/unused-metric"},
		{"never-firing-alert", findings.ClassNeverFiringAlert, "https://remetric.dev/findings/never-firing-alert"},
		{"always-firing-alert", findings.ClassAlwaysFiringAlert, "https://remetric.dev/findings/always-firing-alert"},
		{"label-pattern-overly-granular", findings.ClassLabelPatternOverlyGranular, "https://remetric.dev/findings/label-pattern-overly-granular"},
		{"broken-panel", findings.ClassBrokenPanel, "https://remetric.dev/findings/broken-panel"},
		{"empty class returns empty URL", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := findings.DocURL(tt.class); got != tt.want {
				t.Errorf("DocURL(%q) = %q, want %q", tt.class, got, tt.want)
			}
		})
	}
}
