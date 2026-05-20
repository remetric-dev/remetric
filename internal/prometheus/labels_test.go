// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package prometheus_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	prom "github.com/remetric-dev/remetric/internal/prometheus"
)

func TestClient_LabelNamesForMetric(t *testing.T) {
	var rawQuery string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/labels", func(w http.ResponseWriter, r *http.Request) {
		rawQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":["__name__","destination_principal","destination_service","source_principal","response_code"]}`))
	})
	ts := newTestServer(t, mux)
	c, _ := prom.New(ts.URL)

	got, err := c.LabelNamesForMetric(context.Background(), "istio_requests_total")
	if err != nil {
		t.Fatalf("err = %v", err)
	}

	v, _ := url.ParseQuery(rawQuery)
	if !strings.Contains(v.Get("match[]"), `__name__="istio_requests_total"`) {
		t.Errorf("match[] = %q, missing __name__ matcher", v.Get("match[]"))
	}

	for _, want := range []string{"destination_principal", "response_code"} {
		found := false
		for _, n := range got {
			if n == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing label %q in result %v", want, got)
		}
	}
	for _, n := range got {
		if n == "__name__" {
			t.Errorf("__name__ should be filtered out from result")
		}
	}
}

func TestClient_LabelValues_AppendsMatchQuery(t *testing.T) {
	var rawQuery string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/label/destination_principal/values", func(w http.ResponseWriter, r *http.Request) {
		rawQuery = r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":["a","b","c"]}`))
	})
	ts := newTestServer(t, mux)
	c, _ := prom.New(ts.URL)

	got, err := c.LabelValues(context.Background(), "destination_principal", `{__name__="istio_requests_total"}`)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 3 {
		t.Errorf("len = %d, want 3", len(got))
	}
	v, _ := url.ParseQuery(rawQuery)
	if !strings.Contains(v.Get("match[]"), `__name__="istio_requests_total"`) {
		t.Errorf("match[] = %q, missing __name__ matcher", v.Get("match[]"))
	}
}

func TestClient_LabelValues_NoMatchers(t *testing.T) {
	var requestPath string
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/label/job/values", func(w http.ResponseWriter, r *http.Request) {
		requestPath = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":["prometheus","node"]}`))
	})
	ts := newTestServer(t, mux)
	c, _ := prom.New(ts.URL)

	got, err := c.LabelValues(context.Background(), "job")
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	if len(got) != 2 {
		t.Errorf("len = %d, want 2", len(got))
	}
	if requestPath != "/api/v1/label/job/values" {
		t.Errorf("request path = %q, want bare /api/v1/label/job/values without query string", requestPath)
	}
}

func TestMetricNamesWithLabel(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/label/__name__/values" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		if got := r.URL.Query().Get("match[]"); got != `{user_id!=""}` {
			t.Fatalf("match[] = %q, want %q", got, `{user_id!=""}`)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":["http_requests_total","istio_requests_total"]}`))
	}))
	defer ts.Close()

	c, err := prom.New(ts.URL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	got, err := c.MetricNamesWithLabel(context.Background(), "user_id")
	if err != nil {
		t.Fatalf("MetricNamesWithLabel: %v", err)
	}
	want := []string{"http_requests_total", "istio_requests_total"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("MetricNamesWithLabel mismatch (-want +got):\n%s", diff)
	}
}

func TestMetricNamesWithLabel_EmptyLabel(t *testing.T) {
	c, err := prom.New("http://example.invalid")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = c.MetricNamesWithLabel(context.Background(), "")
	if !errors.Is(err, prom.ErrInvalidArgument) {
		t.Errorf("error = %v, want ErrInvalidArgument", err)
	}
}

func newTestServer(t *testing.T, h http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}
