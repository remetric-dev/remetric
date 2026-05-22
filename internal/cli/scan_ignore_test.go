// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package cli_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	cli "github.com/remetric-dev/remetric/internal/cli"
)

// TestScan_IgnoreMetricDropsFindingAndSetsCount asserts that --ignore-metric
// drops at least one finding from the rendered Report and that the JSON
// envelope exposes ignored_count >= 1. The stub yields high_card_metric
// (cardinality), unreferenced_metric (unused), and user_id (label pattern);
// ignoring high_card_metric must remove cardinality findings for that
// metric while leaving the others intact.
func TestScan_IgnoreMetricDropsFindingAndSetsCount(t *testing.T) {
	ts := newPromScanStub(t)

	var out bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args: []string{
			"scan",
			"--prometheus", ts.URL,
			"--output", "json",
			"--min-severity", "low",
			"--ignore-metric", "high_card_metric",
		},
		Stdout: &out,
		Stderr: &out,
	})
	if code != 0 {
		t.Fatalf("non-zero exit: %d\n%s", code, out.String())
	}

	var rep struct {
		Findings []struct {
			Metric string `json:"metric"`
		} `json:"findings"`
		IgnoredCount int `json:"ignored_count"`
	}
	if err := json.Unmarshal(out.Bytes(), &rep); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, out.String())
	}
	if rep.IgnoredCount < 1 {
		t.Errorf("ignored_count = %d, want >= 1\n%s", rep.IgnoredCount, out.String())
	}
	for _, f := range rep.Findings {
		if f.Metric == "high_card_metric" {
			t.Errorf("findings still contains ignored metric high_card_metric:\n%s", out.String())
		}
	}
}

// TestScan_IgnoreCriticalAvoidsFailOn asserts that an ignored critical
// finding does NOT trip --fail-on=critical. Without --ignore-metric the
// stub yields a critical cardinality finding for high_card_metric and
// the matching baseline test (TestScan_FailOnCriticalExits3...) exits 3.
// With high_card_metric ignored, exit must be 0.
func TestScan_IgnoreCriticalAvoidsFailOn(t *testing.T) {
	ts := newPromScanStub(t)

	var stdout, stderr bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args: []string{
			"scan",
			"--prometheus", ts.URL,
			"--fail-on", "critical",
			// Stub yields critical findings on both high_card_metric (cardinality)
			// and user_id (label-pattern). Ignore both so no critical remains.
			"--ignore-metric", "high_card_metric",
			"--ignore-label", "user_id",
		},
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (ignored critical should not trip --fail-on)\nstdout=%s\nstderr=%s",
			code, stdout.String(), stderr.String())
	}
	// The terminal footer should announce ignored findings.
	if !strings.Contains(stdout.String(), "ignored:") {
		t.Errorf("expected 'ignored:' footer in terminal output:\n%s", stdout.String())
	}
}
