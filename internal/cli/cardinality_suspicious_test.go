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

func newSuspiciousFixtureServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
}

func runSuspiciousCmd(t *testing.T, args []string) (string, int) {
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

func TestCardinalitySuspicious_Terminal(t *testing.T) {
	ts := newSuspiciousFixtureServer(t)
	defer ts.Close()

	out, code := runSuspiciousCmd(t, []string{
		"cardinality", "suspicious",
		"--prometheus", ts.URL,
		"--min-severity", "low",
	})
	if code != 0 {
		t.Fatalf("non-zero exit: %d\n%s", code, out)
	}
	if !strings.Contains(out, "user_id") {
		t.Errorf("expected user_id in output:\n%s", out)
	}
}

func TestCardinalitySuspicious_JSON(t *testing.T) {
	ts := newSuspiciousFixtureServer(t)
	defer ts.Close()

	out, code := runSuspiciousCmd(t, []string{
		"cardinality", "suspicious",
		"--prometheus", ts.URL,
		"--output", "json",
		"--min-severity", "low",
	})
	if code != 0 {
		t.Fatalf("non-zero exit: %d\n%s", code, out)
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, out)
	}
	fs, _ := got["findings"].([]any)
	if len(fs) != 1 {
		t.Errorf("findings length = %d, want 1:\n%s", len(fs), out)
	}
}
