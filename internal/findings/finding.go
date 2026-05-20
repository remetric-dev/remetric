package findings

// Category groups findings by the kind of waste they identify.
type Category string

// Known finding categories.
const (
	CategoryCardinality     Category = "cardinality"
	CategoryUnusedMetrics   Category = "unused_metrics"
	CategoryLabelPatterns   Category = "label_patterns"
	CategoryAlertHygiene    Category = "alert_hygiene"
	CategoryDashboardSprawl Category = "dashboard_sprawl"
)

// Finding is a single piece of feedback produced by an analyzer.
type Finding struct {
	ID       string
	Severity Severity
	Category Category
	Title    string
	Metric   string
	Evidence Evidence
	Fix      Fix
	Impact   Impact
	DocURL   string
}

// Evidence describes the observation that triggered a finding.
type Evidence struct {
	Label        string
	UniqueValues int
	SampleValues []string
	SeriesCount  int64
	Description  string
}

// Fix is a recommended remediation, ready to paste into the user's config.
type Fix struct {
	// Type is one of "scrape_config", "drop_metric", "drop_label", "rule_change".
	Type   string
	Config string
	DocURL string
}

// Impact is the estimated effect of applying Fix.
type Impact struct {
	SeriesReduction      int64
	Percentage           float64
	MemoryReductionBytes int64
	// EstimationMethod is a short identifier for how the numbers were derived,
	// e.g. "labeldrop_upper_bound". Surfaced in JSON output (later phases).
	EstimationMethod string
}
