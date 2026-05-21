// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package findings

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestDoctorReport_JSONShape(t *testing.T) {
	rep := DoctorReport{
		PrometheusURL:        "http://prom:9090",
		Backend:              "prometheus",
		Reachable:            true,
		Version:              "2.51.2",
		VersionOK:            true,
		AuthMethod:           "none",
		TSDBStatsOK:          true,
		NumSeries:            1_234_567,
		NumMetrics:           4_127,
		StorageRetention:     "15d",
		RuntimeInfoSupported: true,
		ElapsedMs:            27,
	}
	b, err := json.Marshal(rep)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var got map[string]any
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	want := map[string]any{
		"prometheus_url":         "http://prom:9090",
		"backend":                "prometheus",
		"reachable":              true,
		"version":                "2.51.2",
		"version_ok":             true,
		"auth_method":            "none",
		"tsdb_stats_accessible":  true,
		"num_series":             float64(1_234_567),
		"num_metrics":            float64(4_127),
		"storage_retention":      "15d",
		"runtime_info_supported": true,
		"elapsed_ms":             float64(27),
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("DoctorReport JSON mismatch (-want +got):\n%s", diff)
	}
}

func TestDoctorReport_OmitsZeros(t *testing.T) {
	rep := DoctorReport{
		PrometheusURL: "http://prom:9090",
		Reachable:     true,
		// numeric fields zero → should be omitted
	}
	b, err := json.Marshal(rep)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	s := string(b)
	for _, banned := range []string{`"num_series"`, `"num_metrics"`, `"storage_retention"`, `"version"`, `"auth_method"`} {
		if strings.Contains(s, banned) {
			t.Errorf("expected %s to be omitted, got: %s", banned, s)
		}
	}
}
