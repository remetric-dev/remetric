// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

// Package json renders findings, reports, and label inventories as JSON.
package json

// LabelInventory is the JSON shape for `cardinality labels --metric X`.
type LabelInventory struct {
	Metric string      `json:"metric"`
	Labels []LabelStat `json:"labels"`
}

// LabelStat is a single label's stats inside a LabelInventory.
type LabelStat struct {
	Name         string   `json:"name"`
	UniqueValues int      `json:"unique_values"`
	SampleValues []string `json:"sample_values,omitempty"`
}
