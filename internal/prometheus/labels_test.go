// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package prometheus_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

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

func newTestServer(t *testing.T, h http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}
