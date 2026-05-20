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

func newPromScanStub(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/-/healthy":
			w.WriteHeader(http.StatusOK)
		case r.URL.Path == "/api/v1/status/buildinfo":
			_, _ = w.Write([]byte(`{"status":"success","data":{"version":"2.51.2"}}`))
		case r.URL.Path == "/api/v1/label/__name__/values" && r.URL.Query().Get("match[]") == `{user_id!=""}`:
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": []string{"high_card_metric"}})
		case r.URL.Path == "/api/v1/label/__name__/values":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   []string{"high_card_metric", "unreferenced_metric"},
			})
		case strings.HasPrefix(r.URL.Path, "/api/v1/status/tsdb"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"headStats": map[string]any{"numSeries": 1_000_000},
					"seriesCountByMetricName": []map[string]any{
						{"name": "high_card_metric", "value": 500_000},
						{"name": "unreferenced_metric", "value": 50_000},
					},
					"labelValueCountByLabelName": []map[string]any{
						{"name": "user_id", "value": 10_000},
					},
				},
			})
		case r.URL.Path == "/api/v1/rules":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": map[string]any{"groups": []map[string]any{}}})
		case r.URL.Path == "/api/v1/labels":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": []string{"__name__", "user_id"}})
		case r.URL.Path == "/api/v1/label/user_id/values":
			_ = json.NewEncoder(w).Encode(map[string]any{"status": "success", "data": []string{"u-1"}})
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestScan_RequiresPrometheus(t *testing.T) {
	var out bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args:    []string{"scan"},
		Stdout:  &out,
		Stderr:  &out,
	})
	if code == 0 {
		t.Errorf("expected non-zero exit for missing --prometheus")
	}
}

func TestScan_JSONReport(t *testing.T) {
	ts := newPromScanStub(t)
	defer ts.Close()

	var out bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args: []string{
			"scan",
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
	body := out.String()
	for _, want := range []string{`"findings"`, `"target"`, `"overview"`, `"summary"`, `"version"`} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %q in JSON output:\n%s", want, body)
		}
	}
}

func TestScan_SkipsUnusedWhenNoGrafana(t *testing.T) {
	ts := newPromScanStub(t)
	defer ts.Close()
	var out bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args: []string{
			"scan",
			"--prometheus", ts.URL,
			"--min-severity", "low",
		},
		Stdout: &out,
		Stderr: &out,
	})
	if code != 0 {
		t.Fatalf("non-zero exit: %d\n%s", code, out.String())
	}
	// scan runs cardinality + labelpattern + unusedmetrics (no Grafana = rules-only).
	// The terminal output should contain at least one finding metric name.
	if !strings.Contains(out.String(), "high_card_metric") && !strings.Contains(out.String(), "user_id") {
		t.Errorf("expected at least one analyzer finding in output:\n%s", out.String())
	}
}
