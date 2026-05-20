// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package labelpattern

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/remetric-dev/remetric/internal/findings"
)

func TestEstimateImpact(t *testing.T) {
	cases := []struct {
		name           string
		seriesAffected int64
		unique         int
		want           findings.Impact
	}{
		{
			name:           "ten_unique_values",
			seriesAffected: 10_000,
			unique:         10,
			// upper bound: 10000 * (1 - 1/10) = 9000
			want: findings.Impact{
				SeriesReduction:  9000,
				Percentage:       90,
				EstimationMethod: "labeldrop_upper_bound",
			},
		},
		{
			name:           "two_unique",
			seriesAffected: 2_000,
			unique:         2,
			want: findings.Impact{
				SeriesReduction:  1000,
				Percentage:       50,
				EstimationMethod: "labeldrop_upper_bound",
			},
		},
		{
			name:           "zero_unique_returns_zero",
			seriesAffected: 1_000,
			unique:         0,
			want: findings.Impact{
				SeriesReduction:  0,
				Percentage:       0,
				EstimationMethod: "labeldrop_upper_bound",
			},
		},
		{
			name:           "one_unique_no_reduction",
			seriesAffected: 1_000,
			unique:         1,
			want: findings.Impact{
				SeriesReduction:  0,
				Percentage:       0,
				EstimationMethod: "labeldrop_upper_bound",
			},
		},
		{
			name:           "zero_series",
			seriesAffected: 0,
			unique:         100,
			want: findings.Impact{
				SeriesReduction:  0,
				Percentage:       0,
				EstimationMethod: "labeldrop_upper_bound",
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := EstimateImpact(c.seriesAffected, c.unique)
			if diff := cmp.Diff(c.want, got, cmpopts.EquateApprox(0, 0.001)); diff != "" {
				t.Errorf("EstimateImpact mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
