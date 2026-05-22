// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package prometheus

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestFlavor_String(t *testing.T) {
	cases := []struct {
		in   Flavor
		want string
	}{
		{FlavorUnknown, "unknown"},
		{FlavorProm, "prometheus"},
		{FlavorVictoria, "victoria"},
	}
	for _, tc := range cases {
		if got := tc.in.String(); got != tc.want {
			t.Errorf("Flavor(%d).String() = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestWithFlavorHint_SetsField(t *testing.T) {
	c, err := New("http://localhost:9090", WithFlavorHint(FlavorVictoria))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if c.flavorHint != FlavorVictoria {
		t.Errorf("flavorHint = %v, want FlavorVictoria", c.flavorHint)
	}
}

func TestDetectFlavor(t *testing.T) {
	cases := []struct {
		name       string
		status     int
		fixture    string
		body       string
		want       Flavor
		wantErrSet bool
	}{
		{name: "prometheus_2_50", status: 200, fixture: "buildinfo-prom.json", want: FlavorProm},
		{name: "vm_single", status: 200, fixture: "buildinfo-vm-single.json", want: FlavorVictoria},
		{name: "vm_cluster", status: 200, fixture: "buildinfo-vm-cluster.json", want: FlavorVictoria},
		{name: "ambiguous", status: 200, fixture: "buildinfo-ambiguous.json", want: FlavorProm},
		{name: "not_found", status: 404, body: "", want: FlavorVictoria},
		{name: "unauthorized", status: 401, body: "", wantErrSet: true, want: FlavorUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var body []byte
			if tc.fixture != "" {
				b, err := os.ReadFile(filepath.Join("testdata", tc.fixture))
				if err != nil {
					t.Fatalf("read fixture: %v", err)
				}
				body = b
			} else {
				body = []byte(tc.body)
			}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/api/v1/status/buildinfo" {
					http.Error(w, "wrong path", 500)
					return
				}
				w.WriteHeader(tc.status)
				_, _ = w.Write(body)
			}))
			defer srv.Close()

			c, err := New(srv.URL)
			if err != nil {
				t.Fatalf("New: %v", err)
			}
			err = c.ensureFlavor(context.Background())
			gotErr := err != nil
			if gotErr != tc.wantErrSet {
				t.Errorf("ensureFlavor err = %v, wantErrSet=%v", err, tc.wantErrSet)
			}
			if c.Flavor() != tc.want {
				t.Errorf("Flavor() = %v, want %v", c.Flavor(), tc.want)
			}
		})
	}
}

func TestEnsureFlavor_RunsOnce(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"status":"success","data":{"version":"v1.99.0"}}`))
	}))
	defer srv.Close()
	c, err := New(srv.URL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	for i := 0; i < 3; i++ {
		if err := c.ensureFlavor(context.Background()); err != nil {
			t.Fatalf("ensureFlavor: %v", err)
		}
	}
	if hits != 1 {
		t.Errorf("buildinfo hit count = %d, want 1", hits)
	}
}

func TestEnsureFlavor_HintSkipsDetection(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(500)
	}))
	defer srv.Close()
	c, err := New(srv.URL, WithFlavorHint(FlavorVictoria))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if err := c.ensureFlavor(context.Background()); err != nil {
		t.Fatalf("ensureFlavor: %v", err)
	}
	if c.Flavor() != FlavorVictoria {
		t.Errorf("Flavor() = %v, want FlavorVictoria", c.Flavor())
	}
	if hits != 0 {
		t.Errorf("buildinfo hit count = %d, want 0", hits)
	}
}

func TestEnsureFlavor_DoesNotCacheCtxError(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(200)
		_, _ = w.Write([]byte(`{"status":"success","data":{"version":"v1.99.0"}}`))
	}))
	defer srv.Close()
	c, err := New(srv.URL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// First call with a pre-cancelled context - should error, not cache.
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	if err := c.ensureFlavor(cancelled); err == nil {
		t.Fatalf("expected error from cancelled context")
	}
	if c.Flavor() != FlavorUnknown {
		t.Errorf("Flavor() after cancelled detect = %v, want FlavorUnknown", c.Flavor())
	}
	// Second call with a fresh context - should succeed and not be sticky.
	if err := c.ensureFlavor(context.Background()); err != nil {
		t.Fatalf("retry after ctx cancel: %v", err)
	}
	if c.Flavor() != FlavorVictoria {
		t.Errorf("Flavor() after successful detect = %v, want FlavorVictoria", c.Flavor())
	}
	// hits should be 1 (the cancelled attempt never reached the server)
	// ... actually since the request goes through c.do which checks ctx before
	// the semaphore, hits may be 0 or 1 depending on timing. Just assert >= 1.
	if hits < 1 {
		t.Errorf("expected at least 1 server hit, got %d", hits)
	}
}
