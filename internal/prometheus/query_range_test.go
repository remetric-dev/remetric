// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package prometheus_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	prom "github.com/remetric-dev/remetric/internal/prometheus"
)

func TestSamplePair_UnmarshalJSON(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want prom.SamplePair
	}{
		{
			name: "integer ts and string value",
			in:   `[1715000000, "1"]`,
			want: prom.SamplePair{Timestamp: time.UnixMilli(1715000000000).UTC(), Value: 1},
		},
		{
			name: "float ts and float string value",
			in:   `[1715000000.500, "0.5"]`,
			want: prom.SamplePair{Timestamp: time.UnixMilli(1715000000500).UTC(), Value: 0.5},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got prom.SamplePair
			if err := json.Unmarshal([]byte(tt.in), &got); err != nil {
				t.Fatalf("Unmarshal: %v", err)
			}
			if diff := cmp.Diff(tt.want, got); diff != "" {
				t.Errorf("SamplePair mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestSamplePair_UnmarshalJSON_Errors(t *testing.T) {
	tests := []struct{ name, in string }{
		{"empty array", `[]`},
		{"missing value", `[1715000000]`},
		{"non-numeric ts", `["abc", "1"]`},
		{"non-string value", `[1715000000, 1]`},
		{"unparseable value", `[1715000000, "not-a-number"]`},
		{"too many elements", `[1715000000, "1", 3]`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got prom.SamplePair
			if err := json.Unmarshal([]byte(tt.in), &got); err == nil {
				t.Errorf("Unmarshal(%q) = nil, want error", tt.in)
			}
		})
	}
}

func TestClient_QueryRange_Parses(t *testing.T) {
	var gotPath, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{
			"status":"success",
			"data":{
				"resultType":"matrix",
				"result":[
					{"metric":{"__name__":"ALERTS","alertname":"X","alertstate":"firing"},
					 "values":[[1715000000,"1"],[1715003600,"1"]]}
				]
			}
		}`))
	}))
	t.Cleanup(srv.Close)

	c, err := prom.New(srv.URL, prom.WithFlavorHint(prom.FlavorProm))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	start := time.Unix(1715000000, 0)
	end := time.Unix(1715003600, 0)
	got, err := c.QueryRange(context.Background(), `ALERTS{alertname="X"}`, start, end, time.Hour)
	if err != nil {
		t.Fatalf("QueryRange: %v", err)
	}
	if gotPath != "/api/v1/query_range" {
		t.Errorf("path = %q, want /api/v1/query_range", gotPath)
	}
	q, _ := url.ParseQuery(gotQuery)
	if q.Get("query") != `ALERTS{alertname="X"}` {
		t.Errorf("query = %q, want ALERTS{alertname=\"X\"}", q.Get("query"))
	}
	if q.Get("start") != "1715000000" {
		t.Errorf("start = %q, want 1715000000", q.Get("start"))
	}
	if q.Get("end") != "1715003600" {
		t.Errorf("end = %q, want 1715003600", q.Get("end"))
	}
	if q.Get("step") != "3600" {
		t.Errorf("step = %q, want 3600", q.Get("step"))
	}
	if got.ResultType != "matrix" {
		t.Errorf("resultType = %q, want matrix", got.ResultType)
	}
	if len(got.Result) != 1 || len(got.Result[0].Values) != 2 {
		t.Fatalf("Result shape = %+v", got.Result)
	}
}

func TestClient_QueryRange_InvalidStep(t *testing.T) {
	c, err := prom.New("http://example.invalid", prom.WithFlavorHint(prom.FlavorProm))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	_, err = c.QueryRange(context.Background(), "up", time.Unix(0, 0), time.Unix(60, 0), 0)
	if err == nil {
		t.Fatalf("QueryRange(step=0) = nil err, want error")
	}
}
