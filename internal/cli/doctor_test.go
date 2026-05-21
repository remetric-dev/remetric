// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package cli_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/remetric-dev/remetric/internal/cli"
)

func newPromStub(t *testing.T, version string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/-/healthy", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
	mux.HandleFunc("/api/v1/status/buildinfo", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"version":"` + version + `","revision":"x","goVersion":"go1.22.0","buildUser":"","buildDate":""}}`))
	})
	mux.HandleFunc("/api/v1/status/tsdb", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"headStats":{"numSeries":100,"numLabelPairs":0,"chunkCount":0,"minTime":0,"maxTime":0},"seriesCountByMetricName":[],"labelValueCountByLabelName":[],"memoryInBytesByLabelName":[],"seriesCountByLabelValuePair":[]}}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestDoctor_Happy(t *testing.T) {
	srv := newPromStub(t, "2.51.2")
	var out bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args:    []string{"doctor", "--prometheus", srv.URL, "--no-color"},
		Stdout:  &out,
		Stderr:  &out,
	})
	if code != 0 {
		t.Errorf("exit = %d, want 0\nout:\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "[OK]") {
		t.Errorf("doctor output missing [OK]:\n%s", out.String())
	}
}

func TestDoctor_OldVersion(t *testing.T) {
	srv := newPromStub(t, "2.29.1")
	var out bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args:    []string{"doctor", "--prometheus", srv.URL, "--no-color"},
		Stdout:  &out,
		Stderr:  &out,
	})
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(out.String(), "below minimum") {
		t.Errorf("output missing minimum-version message:\n%s", out.String())
	}
}

func TestDoctor_Unreachable(t *testing.T) {
	var out bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args:    []string{"doctor", "--prometheus", "http://127.0.0.1:1", "--no-color", "--timeout", "2s"},
		Stdout:  &out,
		Stderr:  &out,
	})
	if code != 1 {
		t.Errorf("exit = %d, want 1", code)
	}
	if !strings.Contains(out.String(), "[FAIL]") {
		t.Errorf("output missing [FAIL]:\n%s", out.String())
	}
}

func TestDoctor_MissingURL(t *testing.T) {
	var out bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args:    []string{"doctor", "--no-color"},
		Stdout:  &out,
		Stderr:  &out,
	})
	if code != 2 {
		t.Errorf("exit = %d, want 2 (invalid flags)", code)
	}
}

func TestDoctor_PopulatesMetricsAndRetention(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/-/healthy":
			w.WriteHeader(http.StatusOK)
		case "/api/v1/status/buildinfo":
			_, _ = w.Write([]byte(`{"status":"success","data":{"version":"2.51.2","revision":"x","goVersion":"go1.22.0"}}`))
		case "/api/v1/status/tsdb":
			_, _ = w.Write([]byte(`{"status":"success","data":{"headStats":{"numSeries":1234},"seriesCountByMetricName":[],"labelValueCountByLabelName":[]}}`))
		case "/api/v1/status/runtimeinfo":
			_, _ = w.Write([]byte(`{"status":"success","data":{"storageRetention":"15d"}}`))
		case "/api/v1/label/__name__/values":
			_, _ = w.Write([]byte(`{"status":"success","data":["a","b","c"]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args:    []string{"doctor", "--prometheus", ts.URL, "--no-color"},
		Stdout:  &out,
		Stderr:  &out,
	})
	if code != 0 {
		t.Fatalf("non-zero exit: %d\n%s", code, out.String())
	}
	body := out.String()
	if !strings.Contains(body, "metric names: 3") {
		t.Errorf("expected metric names line, got:\n%s", body)
	}
	if !strings.Contains(body, "retention:    15d") {
		t.Errorf("expected retention line, got:\n%s", body)
	}
}

func TestDoctor_VictoriaBackend_PrintsLabelAndRuntimeNA(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/-/healthy":
			w.WriteHeader(http.StatusOK)
		case "/api/v1/status/buildinfo":
			// Empty data is a VictoriaMetrics tell, but we also pass
			// --backend=victoria so detection is skipped via the hint.
			_, _ = w.Write([]byte(`{"status":"success","data":{"version":"v1.99.0"}}`))
		case "/api/v1/status/tsdb":
			_, _ = w.Write([]byte(`{"status":"success","data":{"seriesCountByMetricName":[]}}`))
		case "/api/v1/label/__name__/values":
			_, _ = w.Write([]byte(`{"status":"success","data":[]}`))
		case "/api/v1/status/runtimeinfo":
			http.Error(w, "not found", http.StatusNotFound)
		default:
			http.Error(w, "unexpected: "+r.URL.Path, http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	var out bytes.Buffer
	// The Prometheus minimum-version gate is skipped on VictoriaMetrics,
	// so a healthy VM target returns exit code 0 even though VM's "v1.x"
	// version line would not satisfy Prometheus's "2.30+" heuristic.
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args:    []string{"doctor", "--prometheus", srv.URL, "--backend", "victoria", "--no-color"},
		Stdout:  &out,
		Stderr:  &out,
	})
	if code != 0 {
		t.Errorf("exit = %d, want 0\nout:\n%s", code, out.String())
	}
	body := out.String()
	if !strings.Contains(body, "backend:      victoria") {
		t.Errorf("doctor output missing 'backend: victoria' line:\n%s", body)
	}
	if !strings.Contains(body, "retention:    n/a") {
		t.Errorf("doctor output missing 'retention: n/a' line for VM:\n%s", body)
	}
}

func TestDoctor_JSONOutput(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/-/healthy":
			w.WriteHeader(http.StatusOK)
		case "/api/v1/status/buildinfo":
			_, _ = w.Write([]byte(`{"status":"success","data":{"version":"2.51.2","revision":"x","goVersion":"go1.22.0"}}`))
		case "/api/v1/status/tsdb":
			_, _ = w.Write([]byte(`{"status":"success","data":{"headStats":{"numSeries":1234},"seriesCountByMetricName":[],"labelValueCountByLabelName":[]}}`))
		case "/api/v1/status/runtimeinfo":
			_, _ = w.Write([]byte(`{"status":"success","data":{"storageRetention":"15d"}}`))
		case "/api/v1/label/__name__/values":
			_, _ = w.Write([]byte(`{"status":"success","data":["a","b","c"]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer ts.Close()

	var out bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args:    []string{"doctor", "--prometheus", ts.URL, "--output", "json"},
		Stdout:  &out,
		Stderr:  &out,
	})
	if code != 0 {
		t.Fatalf("non-zero exit: %d\n%s", code, out.String())
	}
	body := out.String()
	for _, want := range []string{
		`"prometheus_url"`,
		`"reachable": true`,
		`"version": "2.51.2"`,
		`"num_series": 1234`,
		`"num_metrics": 3`,
		`"storage_retention": "15d"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %q in JSON output, got:\n%s", want, body)
		}
	}
}
