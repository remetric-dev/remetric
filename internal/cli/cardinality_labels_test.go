// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newLabelsFixtureServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/labels":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   []string{"__name__", "user_id", "status"},
			})
		case "/api/v1/label/user_id/values":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   []string{"u-1", "u-2", "u-3"},
			})
		case "/api/v1/label/status/values":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   []string{"200", "500"},
			})
		case "/-/healthy":
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
}

// runLabelsCmd invokes cli.ExecuteWith and returns combined stdout+stderr.
func runLabelsCmd(t *testing.T, args []string) (string, int) {
	t.Helper()
	var out bytes.Buffer
	code := ExecuteWith(Args{
		Version: "test",
		Args:    args,
		Stdout:  &out,
		Stderr:  &out,
	})
	return out.String(), code
}

func TestCardinalityLabels_RequiresMetric(t *testing.T) {
	out, code := runLabelsCmd(t, []string{
		"cardinality", "labels",
		"--prometheus", "http://example.invalid",
	})
	if code == 0 {
		t.Fatalf("expected non-zero exit code for missing --metric, got 0\nout:\n%s", out)
	}
	if !strings.Contains(out, "metric") {
		t.Errorf("error doesn't mention --metric:\n%s", out)
	}
}

func TestCardinalityLabels_Terminal(t *testing.T) {
	ts := newLabelsFixtureServer(t)
	defer ts.Close()

	out, code := runLabelsCmd(t, []string{
		"cardinality", "labels",
		"--metric", "http_requests_total",
		"--prometheus", ts.URL,
	})
	if code != 0 {
		t.Fatalf("non-zero exit: %d\nout:\n%s", code, out)
	}
	for _, want := range []string{"NAME", "user_id", "status"} {
		if !strings.Contains(strings.ToUpper(out), strings.ToUpper(want)) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	// user_id should appear before status (3 > 2 unique values).
	if strings.Index(out, "user_id") > strings.Index(out, "status") {
		t.Errorf("user_id should appear before status in output:\n%s", out)
	}
}

func TestCardinalityLabels_JSON(t *testing.T) {
	ts := newLabelsFixtureServer(t)
	defer ts.Close()

	out, code := runLabelsCmd(t, []string{
		"cardinality", "labels",
		"--metric", "http_requests_total",
		"--prometheus", ts.URL,
		"--output", "json",
	})
	if code != 0 {
		t.Fatalf("non-zero exit: %d\nout:\n%s", code, out)
	}

	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, out)
	}
	if got["metric"] != "http_requests_total" {
		t.Errorf("metric = %v, want http_requests_total", got["metric"])
	}
	labels, _ := got["labels"].([]any)
	if len(labels) != 2 {
		t.Errorf("labels length = %d, want 2:\n%s", len(labels), out)
	}
}
