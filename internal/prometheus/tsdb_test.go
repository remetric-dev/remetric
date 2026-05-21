// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package prometheus_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/google/go-cmp/cmp"

	prom "github.com/remetric-dev/remetric/internal/prometheus"
	"github.com/remetric-dev/remetric/internal/prometheus/promtest"
)

func TestClient_Ping_Healthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/-/healthy" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(200)
	}))
	defer srv.Close()
	c, _ := prom.New(srv.URL)
	if err := c.Ping(context.Background()); err != nil {
		t.Errorf("Ping err = %v, want nil", err)
	}
}

func TestClient_BuildInfo_Parses(t *testing.T) {
	srv := promtest.NewServer(t, "testdata", promtest.Routes{
		"/api/v1/status/buildinfo": "buildinfo_2.51.json",
	})
	c, _ := prom.New(srv.URL)
	got, err := c.BuildInfo(context.Background())
	if err != nil {
		t.Fatalf("BuildInfo err = %v", err)
	}
	want := &prom.BuildInfo{Version: "2.51.2", Revision: "abc", GoVersion: "go1.22.0", BuildUser: "ci", BuildDate: "20240320-00:00:00"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("BuildInfo mismatch (-want +got):\n%s", diff)
	}
}

func TestClient_RuntimeInfo_Parses(t *testing.T) {
	srv := promtest.NewServer(t, "testdata", promtest.Routes{
		"/api/v1/status/runtimeinfo": "runtimeinfo.json",
	})
	c, _ := prom.New(srv.URL, prom.WithFlavorHint(prom.FlavorProm))
	got, err := c.RuntimeInfo(context.Background())
	if err != nil {
		t.Fatalf("RuntimeInfo err = %v", err)
	}
	if got.GoroutineCount != 42 {
		t.Errorf("GoroutineCount = %d, want 42", got.GoroutineCount)
	}
	if got.StorageRetention != "15d" {
		t.Errorf("StorageRetention = %q, want 15d", got.StorageRetention)
	}
}

func TestClient_TSDBStats_Parses(t *testing.T) {
	srv := promtest.NewServer(t, "testdata", promtest.Routes{
		"/api/v1/status/tsdb": "tsdb_stats_typical.json",
	})
	c, _ := prom.New(srv.URL)
	got, err := c.TSDBStats(context.Background(), 20)
	if err != nil {
		t.Fatalf("TSDBStats err = %v", err)
	}
	if got.HeadStats.NumSeries != 2341892 {
		t.Errorf("NumSeries = %d, want 2341892", got.HeadStats.NumSeries)
	}
	if len(got.SeriesCountByMetricName) != 4 {
		t.Errorf("SeriesCountByMetricName len = %d, want 4", len(got.SeriesCountByMetricName))
	}
	if got.SeriesCountByMetricName[0].Name != "istio_requests_total" || got.SeriesCountByMetricName[0].Value != 487234 {
		t.Errorf("top metric = %+v, want istio_requests_total/487234", got.SeriesCountByMetricName[0])
	}
}

func TestClient_Ping_Unhealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()
	c, _ := prom.New(srv.URL)
	if err := c.Ping(context.Background()); err == nil {
		t.Errorf("Ping against 503 returned nil, want error")
	}
}

func TestClient_BuildInfo_Auth401Wraps(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
	}))
	defer srv.Close()
	c, _ := prom.New(srv.URL)
	_, err := c.BuildInfo(context.Background())
	if !errors.Is(err, prom.ErrAuth) {
		t.Errorf("err = %v, want wraps ErrAuth", err)
	}
}

func TestBuildInfo_VMSingleBinary(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/status/buildinfo" {
			http.Error(w, "wrong path", 500)
			return
		}
		_, _ = w.Write([]byte(`{"status":"success","data":{"version":"v1.99.0"}}`))
	}))
	defer srv.Close()
	c, err := prom.New(srv.URL, prom.WithFlavorHint(prom.FlavorVictoria))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	bi, err := c.BuildInfo(context.Background())
	if err != nil {
		t.Fatalf("BuildInfo: %v", err)
	}
	if bi.Version != "v1.99.0" {
		t.Errorf("Version = %q, want %q", bi.Version, "v1.99.0")
	}
	if bi.Revision != "" || bi.GoVersion != "" {
		t.Errorf("VM build info should leave Revision/GoVersion empty, got Revision=%q GoVersion=%q", bi.Revision, bi.GoVersion)
	}
}

func TestBuildInfo_UsesDetectionCache(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = w.Write([]byte(`{"status":"success","data":{"version":"v1.99.0"}}`))
	}))
	defer srv.Close()
	c, err := prom.New(srv.URL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.BuildInfo(context.Background()); err != nil {
		t.Fatalf("BuildInfo: %v", err)
	}
	if _, err := c.BuildInfo(context.Background()); err != nil {
		t.Fatalf("BuildInfo: %v", err)
	}
	if hits != 1 {
		t.Errorf("buildinfo HTTP hits = %d, want 1", hits)
	}
}

func TestBuildInfo_ConcurrentFallbackIsRaceFree(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = w.Write([]byte(`{"status":"success","data":{"version":"v1.99.0"}}`))
	}))
	defer srv.Close()
	c, err := prom.New(srv.URL, prom.WithFlavorHint(prom.FlavorVictoria))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	var wg sync.WaitGroup
	const goroutines = 50
	wg.Add(goroutines)
	results := make([]*prom.BuildInfo, goroutines)
	errs := make([]error, goroutines)
	for i := 0; i < goroutines; i++ {
		go func(i int) {
			defer wg.Done()
			results[i], errs[i] = c.BuildInfo(context.Background())
		}(i)
	}
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Errorf("goroutine %d: %v", i, err)
		}
	}
	// First-writer-wins means all results should point to the same BuildInfo.
	for i := 1; i < goroutines; i++ {
		if results[i] != results[0] {
			t.Errorf("goroutine %d returned different pointer than goroutine 0", i)
			break
		}
	}
	// hits may be >= 1 since the race-free design allows multiple concurrent
	// fetches before the cache is populated. The contract is: first writer wins.
	if hits < 1 {
		t.Errorf("expected at least 1 HTTP hit, got %d", hits)
	}
}

func TestTSDBStats_VictoriaFormat(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("testdata", "tsdb-vm.json"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/status/tsdb" {
			http.Error(w, "wrong path", 500)
			return
		}
		_, _ = w.Write(body)
	}))
	defer srv.Close()
	c, err := prom.New(srv.URL, prom.WithFlavorHint(prom.FlavorVictoria))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	stats, err := c.TSDBStats(context.Background(), 0)
	if err != nil {
		t.Fatalf("TSDBStats: %v", err)
	}
	wantHeadSeries := int64(700) // 500 + 200
	if stats.HeadStats.NumSeries != wantHeadSeries {
		t.Errorf("HeadStats.NumSeries = %d, want %d", stats.HeadStats.NumSeries, wantHeadSeries)
	}
	if len(stats.SeriesCountByMetricName) != 2 {
		t.Errorf("SeriesCountByMetricName len = %d, want 2", len(stats.SeriesCountByMetricName))
	}
	if stats.SeriesCountByMetricName[0].Name != "app_requests_total" {
		t.Errorf("top metric = %q, want %q", stats.SeriesCountByMetricName[0].Name, "app_requests_total")
	}
}

func TestRuntimeInfo_VMReturnsErrNotSupported(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(404)
	}))
	defer srv.Close()
	c, err := prom.New(srv.URL, prom.WithFlavorHint(prom.FlavorVictoria))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = c.RuntimeInfo(context.Background())
	if !errors.Is(err, prom.ErrNotSupported) {
		t.Errorf("err = %v, want ErrNotSupported", err)
	}
	if hits != 0 {
		t.Errorf("RuntimeInfo should not hit the server on VM flavor; hits=%d", hits)
	}
}
