// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package cli_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/remetric-dev/remetric/internal/cli"
)

// TestReport_IgnoreMetricDropsFindingAndSetsCount asserts the same wiring
// for the `report` command: --ignore-metric drops matching findings and
// surfaces ignored_count in the JSON envelope.
func TestReport_IgnoreMetricDropsFindingAndSetsCount(t *testing.T) {
	srv := newReportCriticalStub(t)
	var stdout, stderr bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args: []string{
			"report",
			"--prometheus", srv.URL,
			"--format", "json",
			"--min-severity", "low",
			"--ignore-metric", "high_card_metric",
		},
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("exit = %d, stderr=%s", code, stderr.String())
	}
	var rep struct {
		Findings []struct {
			Metric string `json:"metric"`
		} `json:"findings"`
		IgnoredCount int `json:"ignored_count"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &rep); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, stdout.String())
	}
	if rep.IgnoredCount < 1 {
		t.Errorf("ignored_count = %d, want >= 1\n%s", rep.IgnoredCount, stdout.String())
	}
	for _, f := range rep.Findings {
		if f.Metric == "high_card_metric" {
			t.Errorf("findings still contains ignored metric high_card_metric:\n%s", stdout.String())
		}
	}
}

// TestReport_IgnoreCriticalAvoidsFailOn asserts that an ignored critical
// finding does not trip --fail-on=critical in the report command either.
func TestReport_IgnoreCriticalAvoidsFailOn(t *testing.T) {
	srv := newReportCriticalStub(t)
	var stdout, stderr bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args: []string{
			"report",
			"--prometheus", srv.URL,
			"--fail-on", "critical",
			"--ignore-metric", "high_card_metric",
		},
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (ignored critical should not trip --fail-on)\nstdout=%s\nstderr=%s",
			code, stdout.String(), stderr.String())
	}
}
