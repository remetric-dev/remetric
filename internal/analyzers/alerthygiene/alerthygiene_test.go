// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package alerthygiene

import (
	"testing"
	"time"

	prom "github.com/remetric-dev/remetric/internal/prometheus"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name       string
		series     []prom.Series
		totalSteps int
		want       classification
	}{
		{
			name:       "no series at all",
			series:     nil,
			totalSteps: 168,
			want:       classNeverFired,
		},
		{
			name: "all pending, no firing",
			series: []prom.Series{
				{
					Metric: map[string]string{"alertstate": "pending"},
					Values: []prom.SamplePair{{Value: 1}, {Value: 1}, {Value: 1}},
				},
			},
			totalSteps: 168,
			want:       classNeverFired,
		},
		{
			name: "firing 100% of window",
			series: []prom.Series{
				{
					Metric: map[string]string{"alertstate": "firing"},
					Values: make([]prom.SamplePair, 168),
				},
			},
			totalSteps: 168,
			want:       classAlwaysFiring,
		},
		{
			name: "firing 95% of window (boundary)",
			series: []prom.Series{
				{
					Metric: map[string]string{"alertstate": "firing"},
					Values: make([]prom.SamplePair, 160),
				},
			},
			totalSteps: 168,
			want:       classAlwaysFiring,
		},
		{
			name: "firing 50% — not classified",
			series: []prom.Series{
				{
					Metric: map[string]string{"alertstate": "firing"},
					Values: make([]prom.SamplePair, 84),
				},
			},
			totalSteps: 168,
			want:       classNone,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := classify(tt.series, tt.totalSteps)
			if got != tt.want {
				t.Errorf("classify = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComputeTotalSteps(t *testing.T) {
	tests := []struct {
		lookback time.Duration
		step     time.Duration
		want     int
	}{
		{lookback: 168 * time.Hour, step: time.Hour, want: 168},
		{lookback: time.Hour, step: 30 * time.Minute, want: 2},
		{lookback: 90 * time.Minute, step: time.Hour, want: 2}, // ceil
		{lookback: 0, step: time.Hour, want: 0},
	}
	for _, tt := range tests {
		got := computeTotalSteps(tt.lookback, tt.step)
		if got != tt.want {
			t.Errorf("computeTotalSteps(%v, %v) = %d, want %d", tt.lookback, tt.step, got, tt.want)
		}
	}
}
