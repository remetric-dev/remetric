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

func TestScan_IncludesAlertHygieneRunner(t *testing.T) {
	// Stub returns one alerting rule. Only the alerthygiene analyzer issues
	// ALERTS{alertname=...} via query_range, so queryRangeHit is the
	// load-bearing signal that the analyzer was registered into scan's
	// runner list. (Note: unusedmetrics also calls /api/v1/rules for its
	// rules-only fallback, so rulesHit alone wouldn't discriminate.)
	var rulesHit, queryRangeHit bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/rules":
			rulesHit = true
			_, _ = w.Write([]byte(`{"status":"success","data":{"groups":[{"name":"g","file":"f.yml","rules":[{"name":"NeverFires","type":"alerting","query":"vector(1)"}]}]}}`))
		case "/api/v1/query_range":
			queryRangeHit = true
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`))
		case "/api/v1/label/__name__/values":
			_, _ = w.Write([]byte(`{"status":"success","data":[]}`))
		case "/api/v1/status/tsdb":
			_, _ = w.Write([]byte(`{"status":"success","data":{"headStats":{"numSeries":0},"seriesCountByMetricName":[]}}`))
		case "/api/v1/status/buildinfo":
			_, _ = w.Write([]byte(`{"status":"success","data":{"version":"2.51.2","revision":"abc","goVersion":"go1.22"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	var stdout, stderr bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args:    []string{"scan", "--prometheus", srv.URL, "--output", "json", "--min-severity", "low"},
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	if code != 0 {
		t.Fatalf("exit code = %d, stderr=%s", code, stderr.String())
	}
	if !rulesHit {
		t.Errorf("scan did not call /api/v1/rules")
	}
	if !queryRangeHit {
		t.Errorf("scan did not call /api/v1/query_range — alerthygiene analyzer not wired into scan runners")
	}
	if !strings.Contains(stdout.String(), "NeverFires") {
		t.Errorf("scan output missing NeverFires finding:\n%s", stdout.String())
	}
}

func TestScan_FailOnCriticalExits3WhenCriticalFindingPresent(t *testing.T) {
	ts := newPromScanStub(t)
	defer ts.Close()
	var stdout, stderr bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args: []string{
			"scan",
			"--prometheus", ts.URL,
			"--fail-on", "critical",
		},
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 3 {
		t.Fatalf("exit code = %d, want 3 (stderr=%s)", code, stderr.String())
	}
}

func TestScan_FailOnNoneExits0EvenWithCritical(t *testing.T) {
	ts := newPromScanStub(t)
	defer ts.Close()
	var stdout, stderr bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args: []string{
			"scan",
			"--prometheus", ts.URL,
			"--fail-on", "none",
		},
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr=%s)", code, stderr.String())
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
