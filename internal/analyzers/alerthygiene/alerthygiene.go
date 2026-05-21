// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

// Package alerthygiene flags alerting rules that never fire or fire
// continuously, by inspecting the ALERTS series via query_range.
package alerthygiene

import (
	"math"
	"time"

	prom "github.com/remetric-dev/remetric/internal/prometheus"
)

// alwaysFiringThreshold is the share of evaluation steps with
// alertstate="firing" above which an alert is flagged as always-firing.
const alwaysFiringThreshold = 0.95

// classification is the analyzer's verdict on a single alert.
type classification int

const (
	classNone classification = iota
	classNeverFired
	classAlwaysFiring
)

// classify inspects the ALERTS-series result of one alert over a window
// of totalSteps and returns the classification plus the firing ratio
// (samples with alertstate="firing" divided by totalSteps).
func classify(series []prom.Series, totalSteps int) (classification, float64) {
	if totalSteps <= 0 {
		return classNone, 0
	}
	var firing int
	for _, s := range series {
		if s.Metric["alertstate"] != "firing" {
			continue
		}
		firing += len(s.Values)
	}
	if firing == 0 {
		return classNeverFired, 0
	}
	ratio := float64(firing) / float64(totalSteps)
	if ratio >= alwaysFiringThreshold {
		return classAlwaysFiring, ratio
	}
	return classNone, ratio
}

// computeTotalSteps returns ceil(lookback / step) as a step count.
func computeTotalSteps(lookback, step time.Duration) int {
	if step <= 0 || lookback <= 0 {
		return 0
	}
	return int(math.Ceil(float64(lookback) / float64(step)))
}
