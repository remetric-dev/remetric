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

func TestCardinalitySuspicious_NoResults(t *testing.T) {
	// Fixture: TSDB returns labels but none match suspicious patterns.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/status/tsdb"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"headStats":               map[string]any{"numSeries": 1_000_000},
					"seriesCountByMetricName": []map[string]any{},
					"labelValueCountByLabelName": []map[string]any{
						{"name": "status", "value": 5},
						{"name": "code", "value": 8},
					},
				},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	code := ExecuteWith(Args{
		Version: "test",
		Args:    []string{"cardinality", "suspicious", "--prometheus", ts.URL, "--min-severity", "low"},
		Stdout:  &out,
		Stderr:  &out,
	})
	if code != 0 {
		t.Fatalf("non-zero exit: %d\n%s", code, out.String())
	}
	body := out.String()
	if !strings.Contains(body, "No suspicious labels") {
		t.Errorf("expected NoResults copy, got:\n%s", body)
	}
	if !strings.Contains(body, "remetric doctor") {
		t.Errorf("expected doctor hint, got:\n%s", body)
	}
}

// newSuspiciousCriticalServer is like newSuspiciousFixtureServer but the
// suspicious label has >5000 unique values so the resulting finding is
// classified as Critical (LabelPatternSeverity > 5000).
func newSuspiciousCriticalServer(t *testing.T) *httptest.Server {
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
						{"name": "user_id", "value": 6_000},
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

func TestCardinalitySuspicious_FailOnCriticalExits3WhenCriticalFindingPresent(t *testing.T) {
	ts := newSuspiciousCriticalServer(t)
	defer ts.Close()

	out, code := runSuspiciousCmd(t, []string{
		"cardinality", "suspicious",
		"--prometheus", ts.URL,
		"--fail-on", "critical",
	})
	if code != 3 {
		t.Fatalf("exit code = %d, want 3 (out=%s)", code, out)
	}
}

func TestCardinalitySuspicious_FailOnNoneExits0EvenWithCritical(t *testing.T) {
	ts := newSuspiciousCriticalServer(t)
	defer ts.Close()

	out, code := runSuspiciousCmd(t, []string{
		"cardinality", "suspicious",
		"--prometheus", ts.URL,
		"--fail-on", "none",
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (out=%s)", code, out)
	}
}

func TestCardinalitySuspicious_AllFiltered_ShowsHint(t *testing.T) {
	// Fixture: user_id label with 50 unique values → Low severity.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v1/status/tsdb"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data": map[string]any{
					"headStats":               map[string]any{"numSeries": 1_000_000},
					"seriesCountByMetricName": []map[string]any{{"name": "m", "value": 100}},
					"labelValueCountByLabelName": []map[string]any{
						{"name": "user_id", "value": 50},
					},
				},
			})
		case r.URL.Path == "/api/v1/label/__name__/values":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   []string{"m"},
			})
		case r.URL.Path == "/api/v1/label/user_id/values":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": "success",
				"data":   []string{"u1", "u2"},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	code := ExecuteWith(Args{
		Version: "test",
		Args:    []string{"cardinality", "suspicious", "--prometheus", ts.URL, "--min-severity", "medium"},
		Stdout:  &out,
		Stderr:  &out,
	})
	if code != 0 {
		t.Fatalf("non-zero exit: %d\n%s", code, out.String())
	}
	body := out.String()
	for _, want := range []string{"Filtered 1", "suspicious labels", "1 low", "--min-severity low"} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %q in output:\n%s", want, body)
		}
	}
}
