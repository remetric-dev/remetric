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

// TestCardinalityTop_IgnoreMetricDropsFindingAndSetsCount asserts that
// --ignore-metric drops the matching cardinality finding and that the
// JSON envelope exposes ignored_count >= 1. The cardinality stub yields
// a single critical istio_requests_total finding; ignoring it must leave
// findings empty and surface the count via the empty-state JSON envelope.
func TestCardinalityTop_IgnoreMetricDropsFindingAndSetsCount(t *testing.T) {
	srv := newCardinalityStub(t)

	var out bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args: []string{
			"cardinality", "top",
			"--prometheus", srv.URL,
			"--output", "json",
			"--min-severity", "low",
			"--ignore-metric", "istio_requests_total",
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
		if f.Metric == "istio_requests_total" {
			t.Errorf("findings still contains ignored metric istio_requests_total:\n%s", out.String())
		}
	}
}

// TestCardinalityTop_IgnoreCriticalAvoidsFailOn asserts that an ignored
// critical finding does NOT trip --fail-on=critical. The baseline test
// TestCardinalityTop_FailOnCriticalExits3WhenCriticalFindingPresent exits
// 3 against the same stub; with istio_requests_total ignored, exit must
// be 0 and the terminal footer must announce the ignored count.
func TestCardinalityTop_IgnoreCriticalAvoidsFailOn(t *testing.T) {
	srv := newCardinalityStub(t)
	var stdout, stderr bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args: []string{
			"cardinality", "top",
			"--prometheus", srv.URL,
			"--fail-on", "critical",
			"--no-color",
			"--min-severity", "low",
			"--ignore-metric", "istio_requests_total",
		},
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (ignored critical should not trip --fail-on)\nstdout=%s\nstderr=%s",
			code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "ignored:") {
		t.Errorf("expected 'ignored:' footer in terminal output:\n%s", stdout.String())
	}
}
