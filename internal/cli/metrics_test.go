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

	cli "github.com/remetric-dev/remetric/internal/cli"
)

// newPromUnusedStub returns a Prom server with one ingested metric `nobody_uses_me`.
func newPromUnusedStub(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/-/healthy":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/v1/label/__name__/values":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": []string{"nobody_uses_me"}})
		case strings.HasPrefix(r.URL.Path, "/api/v1/status/tsdb"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"headStats":               map[string]any{"numSeries": 10},
					"seriesCountByMetricName": []map[string]any{{"name": "nobody_uses_me", "value": 10}},
				},
			})
		case r.URL.Path == "/api/v1/rules":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   map[string]any{"groups": []map[string]any{}},
			})
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestMetricsUnused_RequiresPrometheus(t *testing.T) {
	var out bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args:    []string{"metrics", "unused"},
		Stdout:  &out,
		Stderr:  &out,
	})
	if code == 0 {
		t.Fatalf("expected non-zero exit for missing --prometheus, got 0")
	}
	if !strings.Contains(out.String(), "prometheus") {
		t.Errorf("error doesn't mention prometheus: %s", out.String())
	}
}

func TestMetricsUnused_FindsUnusedMetric(t *testing.T) {
	ts := newPromUnusedStub(t)
	defer ts.Close()

	var out bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args: []string{
			"metrics", "unused",
			"--prometheus", ts.URL,
			"--min-severity", "low",
		},
		Stdout: &out,
		Stderr: &out,
	})
	if code != 0 {
		t.Fatalf("non-zero exit: %d\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "nobody_uses_me") {
		t.Errorf("expected metric name in output:\n%s", out.String())
	}
}

func TestMetricsUnused_JSONOutput(t *testing.T) {
	ts := newPromUnusedStub(t)
	defer ts.Close()

	var out bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args: []string{
			"metrics", "unused",
			"--prometheus", ts.URL,
			"--output", "json",
			"--min-severity", "low",
		},
		Stdout: &out,
		Stderr: &out,
	})
	if code != 0 {
		t.Fatalf("non-zero exit: %d\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), `"findings"`) {
		t.Errorf("JSON output missing findings key:\n%s", out.String())
	}
	if !strings.Contains(out.String(), `"metric": "nobody_uses_me"`) {
		t.Errorf("metric not in JSON:\n%s", out.String())
	}
}
