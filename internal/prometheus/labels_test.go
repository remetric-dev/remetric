package prometheus_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	prom "github.com/remetric-dev/remetric/internal/prometheus"
	"github.com/remetric-dev/remetric/internal/prometheus/promtest"
)

func TestClient_LabelNamesForMetric(t *testing.T) {
	srv := promtest.NewServer(t, "testdata", promtest.Routes{
		"/api/v1/labels": "labels_istio.json",
	})
	c, _ := prom.New(srv.URL)
	got, err := c.LabelNamesForMetric(context.Background(), "istio_requests_total")
	if err != nil {
		t.Fatalf("err = %v", err)
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

func newTestServer(t *testing.T, h http.Handler) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	return srv
}
