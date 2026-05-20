// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package findings

// DoctorReport is the result of the `remetric doctor` connectivity check.
// Both the terminal renderer and the JSON renderer consume this type;
// CLI populates it.
type DoctorReport struct {
	PrometheusURL    string        `json:"prometheus_url"`
	Reachable        bool          `json:"reachable"`
	Version          string        `json:"version,omitempty"`
	VersionOK        bool          `json:"version_ok"`
	AuthMethod       string        `json:"auth_method,omitempty"`
	TSDBStatsOK      bool          `json:"tsdb_stats_accessible"`
	NumSeries        int64         `json:"num_series,omitempty"`
	NumMetrics       int64         `json:"num_metrics,omitempty"`
	StorageRetention string        `json:"storage_retention,omitempty"`
	ElapsedMs        int64         `json:"elapsed_ms"`
	Errors           []DoctorError `json:"errors,omitempty"`
}

// DoctorError is a JSON-friendly form of a failed check.
type DoctorError struct {
	Check string `json:"check"`
	Error string `json:"error"`
}
