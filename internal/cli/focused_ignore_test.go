// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package cli_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/remetric-dev/remetric/internal/cli"
)

// newPromSuspiciousStubExt is an externally-visible (cli_test package)
// stub that produces enough state for `cardinality suspicious` to flag
// the `user_id` label. Mirrors the in-package `newSuspiciousFixtureServer`
// at internal/cli/cardinality_suspicious_test.go:15 but lives here so
// tests in package cli_test can reach it.
func newPromSuspiciousStubExt(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/status/tsdb"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"headStats": map[string]any{"numSeries": 1_000_000},
					"seriesCountByMetricName": []map[string]any{
						{"name": "http_requests_total", "value": 800_000},
					},
					"labelValueCountByLabelName": []map[string]any{
						{"name": "user_id", "value": 5_000},
					},
				},
			})
		case r.URL.Path == "/api/v1/label/__name__/values":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   []string{"http_requests_total"},
			})
		case r.URL.Path == "/api/v1/label/user_id/values":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   []string{"u-1", "u-2", "u-3"},
			})
		case r.URL.Path == "/-/healthy":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// runJSONIgnoreCheck runs the given args, parses JSON output, and asserts
// `ignored_count == wantIgnored` plus none of `bannedMetricSubstrings` appear
// in any finding's metric field. Returns the parsed envelope for further
// per-test assertions.
func runJSONIgnoreCheck(t *testing.T, args []string, wantIgnored int, bannedMetricSubstrings ...string) []map[string]any {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args:    args,
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	if code != 0 {
		t.Fatalf("exit = %d, stderr=%s\nstdout=%s", code, stderr.String(), stdout.String())
	}
	var env struct {
		IgnoredCount int              `json:"ignored_count"`
		Findings     []map[string]any `json:"findings"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &env); err != nil {
		t.Fatalf("unmarshal: %v\nout: %s", err, stdout.String())
	}
	if env.IgnoredCount != wantIgnored {
		t.Errorf("ignored_count = %d, want %d", env.IgnoredCount, wantIgnored)
	}
	for _, f := range env.Findings {
		m, _ := f["metric"].(string)
		for _, banned := range bannedMetricSubstrings {
			if banned != "" && strings.Contains(m, banned) {
				t.Errorf("finding leaked metric %q despite ignore: %v", m, f)
			}
		}
	}
	return env.Findings
}

func TestCardinalitySuspicious_IgnoreLabelDropsFinding(t *testing.T) {
	ts := newPromSuspiciousStubExt(t)
	runJSONIgnoreCheck(t,
		[]string{
			"cardinality", "suspicious",
			"--prometheus", ts.URL,
			"--output", "json",
			"--min-severity", "low",
			"--ignore-label", "user_id",
		},
		1, // exactly one suspicious-label finding in the fixture (user_id)
	)
}

func TestMetricsUnused_IgnoreMetricDropsFinding(t *testing.T) {
	srv := newPromUnusedStub(t)
	defer srv.Close()
	runJSONIgnoreCheck(t,
		[]string{
			"metrics", "unused",
			"--prometheus", srv.URL,
			"--output", "json",
			"--min-severity", "low",
			"--ignore-metric", "nobody_uses_me",
		},
		1, // the only unused-metric finding in the fixture
		"nobody_uses_me",
	)
}

func TestAlertsAlwaysFiring_IgnoreAlertDropsFinding(t *testing.T) {
	vals := strings.Repeat(`[1715000000,"1"],`, 60)
	vals = strings.TrimSuffix(vals, ",")
	firingBody := "{\"resultType\":\"matrix\",\"result\":[{\"metric\":{\"alertstate\":\"firing\"},\"values\":[" + vals + "]}]}"
	srv := newAlertsStub(t, map[string]string{"BrokenAlert": firingBody})
	defer srv.Close()
	runJSONIgnoreCheck(t,
		[]string{
			"alerts", "always-firing",
			"--prometheus", srv.URL,
			"--lookback", "1h", "--step", "1m",
			"--output", "json",
			"--ignore-alert", "BrokenAlert",
		},
		1,
	)
}
