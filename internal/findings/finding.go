// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package findings

// Category groups findings by the kind of waste they identify.
type Category string

// Known finding categories.
const (
	CategoryCardinality      Category = "cardinality"
	CategoryUnusedMetrics    Category = "unused_metrics"
	CategoryLabelPatterns    Category = "label_patterns"
	CategoryAlertHygiene     Category = "alert_hygiene"
	CategoryDashboardHygiene Category = "dashboard_hygiene"
)

// Finding is a single piece of feedback produced by an analyzer.
type Finding struct {
	ID        string   `json:"id"`
	Severity  Severity `json:"severity"`
	Category  Category `json:"category"`
	Class     Class    `json:"class,omitempty"`
	Title     string   `json:"title"`
	Metric    string   `json:"metric,omitempty"`
	Alert     string   `json:"alert,omitempty"`
	Dashboard string   `json:"dashboard,omitempty"`
	Evidence  Evidence `json:"evidence"`
	Fix       Fix      `json:"fix"`
	Impact    Impact   `json:"impact"`
	DocURL    string   `json:"documentation_url,omitempty"`
}

// Evidence describes the observation that triggered a finding.
type Evidence struct {
	Label        string   `json:"label,omitempty"`
	UniqueValues int      `json:"unique_values,omitempty"`
	SampleValues []string `json:"sample_values,omitempty"`
	SeriesCount  int64    `json:"series_count,omitempty"`
	Description  string   `json:"description"`
}

// Fix is a recommended remediation, ready to paste into the user's config.
type Fix struct {
	// Type is one of "scrape_config", "drop_metric", "drop_label", "rule_change".
	Type   string `json:"type"`
	Config string `json:"config"`
	DocURL string `json:"documentation_url,omitempty"`
}

// Impact is the estimated effect of applying Fix.
//
// SeriesReduction, Percentage, and Description-style fields render even
// when zero - analyzers always populate them, and explicit zeros are
// part of the wire contract. MemoryReductionBytes and EstimationMethod
// use omitempty because they are optional metadata that not every
// analyzer can compute.
type Impact struct {
	SeriesReduction      int64   `json:"series_reduction"`
	Percentage           float64 `json:"percentage"`
	MemoryReductionBytes int64   `json:"memory_reduction_bytes,omitempty"`
	// EstimationMethod is a short identifier for how the numbers were derived,
	// e.g. "labeldrop_upper_bound".
	EstimationMethod string `json:"estimation_method,omitempty"`
}
