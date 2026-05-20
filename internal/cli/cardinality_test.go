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

func newCardinalityStub(t *testing.T) *httptest.Server {
	t.Helper()
	fixturesDir := filepath.Join("..", "analyzers", "cardinality", "testdata")
	read := func(name string) []byte {
		b, err := os.ReadFile(filepath.Join(fixturesDir, name))
		if err != nil {
			t.Fatalf("read fixture %s: %v", name, err)
		}
		return b
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/status/tsdb", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(read("tsdb_one_bomb.json"))
	})
	mux.HandleFunc("/api/v1/labels", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(read("labels_one_bomb.json"))
	})
	mux.HandleFunc("/api/v1/label/destination_principal/values", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(read("label_values_dp.json"))
	})
	mux.HandleFunc("/api/v1/label/response_code/values", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(read("label_values_rc.json"))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestCardinalityTop_Happy(t *testing.T) {
	srv := newCardinalityStub(t)
	var out bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args:    []string{"cardinality", "top", "--prometheus", srv.URL, "--no-color"},
		Stdout:  &out,
		Stderr:  &out,
	})
	if code != 0 {
		t.Errorf("exit = %d, want 0; out:\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), "istio_requests_total") {
		t.Errorf("output missing top metric:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "CRITICAL") {
		t.Errorf("output missing CRITICAL marker:\n%s", out.String())
	}
}

func TestCardinalityTop_Limit(t *testing.T) {
	srv := newCardinalityStub(t)
	var out bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args:    []string{"cardinality", "top", "--prometheus", srv.URL, "--no-color", "--limit", "0"},
		Stdout:  &out,
		Stderr:  &out,
	})
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "No cardinality findings") {
		t.Errorf("--limit 0 should produce empty result; got:\n%s", out.String())
	}
}

func TestCardinalityTop_MinSeverityFilters(t *testing.T) {
	srv := newCardinalityStub(t)
	var out bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args:    []string{"cardinality", "top", "--prometheus", srv.URL, "--no-color", "--min-severity", "critical"},
		Stdout:  &out,
		Stderr:  &out,
	})
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if strings.Contains(out.String(), "HIGH") {
		t.Errorf("expected no HIGH rows with --min-severity critical:\n%s", out.String())
	}
}

func TestCardinalityTop_JSONOutput(t *testing.T) {
	srv := newCardinalityStub(t)
	var out bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args: []string{
			"cardinality", "top",
			"--prometheus", srv.URL,
			"--output", "json",
			"--min-severity", "low",
		},
		Stdout: &out,
		Stderr: &out,
	})
	if code != 0 {
		t.Fatalf("exit = %d, want 0; out:\n%s", code, out.String())
	}
	if !strings.Contains(out.String(), `"findings"`) {
		t.Errorf("expected JSON findings envelope, got:\n%s", out.String())
	}
}

func TestCardinalityTop_InvalidOutput(t *testing.T) {
	var out bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args: []string{
			"cardinality", "top",
			"--prometheus", "http://example.invalid",
			"--output", "yaml",
		},
		Stdout: &out,
		Stderr: &out,
	})
	if code == 0 {
		t.Fatalf("expected non-zero exit for --output yaml, got 0; out:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "yaml") {
		t.Errorf("error doesn't mention bad value:\n%s", out.String())
	}
}
