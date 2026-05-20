// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package prometheus_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
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
	c, _ := prom.New(srv.URL)
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
