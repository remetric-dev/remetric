// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package cli_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/remetric-dev/remetric/internal/cli"
)

func newReportStub(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/api/v1/status/buildinfo":
			_, _ = w.Write([]byte(`{"status":"success","data":{"version":"2.51.2","revision":"abc","goVersion":"go1.22"}}`))
		case r.URL.Path == "/api/v1/label/__name__/values":
			_, _ = w.Write([]byte(`{"status":"success","data":["up","go_goroutines"]}`))
		case r.URL.Path == "/api/v1/labels":
			_, _ = w.Write([]byte(`{"status":"success","data":["__name__","instance","job"]}`))
		case strings.HasPrefix(r.URL.Path, "/api/v1/label/") && strings.HasSuffix(r.URL.Path, "/values"):
			_, _ = w.Write([]byte(`{"status":"success","data":[]}`))
		case r.URL.Path == "/api/v1/status/tsdb":
			_, _ = w.Write([]byte(`{"status":"success","data":{"headStats":{"numSeries":42},"seriesCountByMetricName":[{"name":"up","value":1},{"name":"go_goroutines","value":41}]}}`))
		case r.URL.Path == "/api/v1/rules":
			_, _ = w.Write([]byte(`{"status":"success","data":{"groups":[]}}`))
		case r.URL.Path == "/api/v1/query_range":
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[]}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestReport_HTMLToFile(t *testing.T) {
	srv := newReportStub(t)
	out := filepath.Join(t.TempDir(), "report.html")
	var stdout, stderr bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args:    []string{"report", "--prometheus", srv.URL, "--format", "html", "--out", out},
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	if code != 0 {
		t.Fatalf("exit = %d, stderr=%s", code, stderr.String())
	}
	body, err := os.ReadFile(out)
	if err != nil {
		t.Fatalf("read %s: %v", out, err)
	}
	if !strings.HasPrefix(strings.TrimSpace(string(body)), "<!DOCTYPE html>") {
		t.Errorf("output not HTML:\n%s", string(body))
	}
}

func TestReport_MarkdownToStdout(t *testing.T) {
	srv := newReportStub(t)
	var stdout, stderr bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args:    []string{"report", "--prometheus", srv.URL, "--format", "markdown"},
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	if code != 0 {
		t.Fatalf("exit = %d, stderr=%s", code, stderr.String())
	}
	if !strings.HasPrefix(stdout.String(), "# remetric report") {
		t.Errorf("stdout not markdown:\n%s", stdout.String())
	}
}

func TestReport_InvalidFormat(t *testing.T) {
	srv := newReportStub(t)
	var stdout, stderr bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args:    []string{"report", "--prometheus", srv.URL, "--format", "xml"},
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}

func TestReport_InvalidGlobalOutputRejected(t *testing.T) {
	srv := newReportStub(t)
	var stdout, stderr bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args:    []string{"report", "--prometheus", srv.URL, "--output", "xml", "--format", "html"},
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	if code != 2 {
		t.Errorf("exit = %d, want 2 (invalid --output should be rejected)", code)
	}
}
