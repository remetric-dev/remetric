// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

// Package promtest provides httptest helpers for Prometheus API mocking.
package promtest

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Routes maps URL path → fixture file path (relative to caller test data).
type Routes map[string]string

// NewServer returns a *httptest.Server that serves the given fixture file
// for each path. Query strings are ignored when matching. Unknown paths
// respond with 404.
func NewServer(t *testing.T, fixturesDir string, routes Routes) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	for path, fixture := range routes {
		fpath := filepath.Join(fixturesDir, fixture)
		body, err := os.ReadFile(fpath) //nolint:gosec // test-only helper; fixturesDir is supplied by the test author
		if err != nil {
			t.Fatalf("promtest: read fixture %q: %v", fpath, err)
		}
		mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) {
			ct := "application/json"
			if strings.HasSuffix(fixture, ".txt") {
				ct = "text/plain"
			}
			w.Header().Set("Content-Type", ct)
			_, _ = w.Write(body)
		})
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}
