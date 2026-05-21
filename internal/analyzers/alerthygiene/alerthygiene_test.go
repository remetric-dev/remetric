// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package alerthygiene

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/remetric-dev/remetric/internal/analyzers"
	"github.com/remetric-dev/remetric/internal/findings"
	prom "github.com/remetric-dev/remetric/internal/prometheus"
)

func TestClassify(t *testing.T) {
	tests := []struct {
		name       string
		series     []prom.Series
		totalSteps int
		want       classification
	}{
		{
			name:       "no series at all",
			series:     nil,
			totalSteps: 168,
			want:       classNeverFired,
		},
		{
			name: "all pending, no firing",
			series: []prom.Series{
				{
					Metric: map[string]string{"alertstate": "pending"},
					Values: []prom.SamplePair{{Value: 1}, {Value: 1}, {Value: 1}},
				},
			},
			totalSteps: 168,
			want:       classNeverFired,
		},
		{
			name: "firing 100% of window",
			series: []prom.Series{
				{
					Metric: map[string]string{"alertstate": "firing"},
					Values: make([]prom.SamplePair, 168),
				},
			},
			totalSteps: 168,
			want:       classAlwaysFiring,
		},
		{
			name: "firing 95% of window (boundary)",
			series: []prom.Series{
				{
					Metric: map[string]string{"alertstate": "firing"},
					Values: make([]prom.SamplePair, 160),
				},
			},
			totalSteps: 168,
			want:       classAlwaysFiring,
		},
		{
			name: "firing 50% — not classified",
			series: []prom.Series{
				{
					Metric: map[string]string{"alertstate": "firing"},
					Values: make([]prom.SamplePair, 84),
				},
			},
			totalSteps: 168,
			want:       classNone,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, _ := classify(tt.series, tt.totalSteps)
			if got != tt.want {
				t.Errorf("classify = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComputeTotalSteps(t *testing.T) {
	tests := []struct {
		lookback time.Duration
		step     time.Duration
		want     int
	}{
		{lookback: 168 * time.Hour, step: time.Hour, want: 168},
		{lookback: time.Hour, step: 30 * time.Minute, want: 2},
		{lookback: 90 * time.Minute, step: time.Hour, want: 2}, // ceil
		{lookback: 0, step: time.Hour, want: 0},
	}
	for _, tt := range tests {
		got := computeTotalSteps(tt.lookback, tt.step)
		if got != tt.want {
			t.Errorf("computeTotalSteps(%v, %v) = %d, want %d", tt.lookback, tt.step, got, tt.want)
		}
	}
}

// newAlertHygieneStub returns a Prom-flavored stub that serves /api/v1/rules
// with the provided alerts and /api/v1/query_range answers via the
// alertname → series map. Unknown alertnames respond with an empty result.
func newAlertHygieneStub(t *testing.T, alerts []string, queryResults map[string]string) (*httptest.Server, *prom.Client) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/rules", func(w http.ResponseWriter, _ *http.Request) {
		var rules strings.Builder
		for i, name := range alerts {
			if i > 0 {
				rules.WriteString(",")
			}
			fmt.Fprintf(&rules, `{"name":%q,"query":"up","type":"alerting"}`, name)
		}
		fmt.Fprintf(w,
			`{"status":"success","data":{"groups":[{"name":"g","file":"f.yml","rules":[%s]}]}}`,
			rules.String())
	})
	mux.HandleFunc("/api/v1/query_range", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("query")
		name := alertNameFromQuery(q)
		body, ok := queryResults[name]
		if !ok {
			body = `{"resultType":"matrix","result":[]}`
		}
		fmt.Fprintf(w, `{"status":"success","data":%s}`, body)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c, err := prom.New(srv.URL, prom.WithFlavorHint(prom.FlavorProm))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return srv, c
}

func alertNameFromQuery(q string) string {
	const marker = `alertname="`
	i := strings.Index(q, marker)
	if i < 0 {
		return ""
	}
	rest := q[i+len(marker):]
	j := strings.Index(rest, `"`)
	if j < 0 {
		return ""
	}
	return rest[:j]
}

func TestAlertHygiene_NeverFired(t *testing.T) {
	_, c := newAlertHygieneStub(t,
		[]string{"NoiseAlert"},
		map[string]string{
			"NoiseAlert": `{"resultType":"matrix","result":[]}`,
		},
	)
	a := New(Config{Lookback: 168 * time.Hour, Step: time.Hour, Now: func() time.Time { return time.Unix(1715000000, 0) }})
	res, err := a.Analyze(context.Background(), analyzers.Deps{Prom: c, Logger: slog.Default()})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("Findings = %d, want 1", len(res.Findings))
	}
	f := res.Findings[0]
	if f.ID != "alert_hygiene/never_fired" {
		t.Errorf("ID = %q, want alert_hygiene/never_fired", f.ID)
	}
	if f.Severity != findings.SeverityMedium {
		t.Errorf("Severity = %v, want Medium", f.Severity)
	}
	if f.Category != findings.CategoryAlertHygiene {
		t.Errorf("Category = %v, want alert_hygiene", f.Category)
	}
	if !strings.Contains(f.Title, "NoiseAlert") {
		t.Errorf("Title = %q, want it to mention NoiseAlert", f.Title)
	}
}

func TestAlertHygiene_AlwaysFiring(t *testing.T) {
	// 168 firing samples over a 168-step window = 100%
	vals := strings.Repeat(`[1715000000,"1"],`, 168)
	vals = strings.TrimSuffix(vals, ",")
	body := fmt.Sprintf(
		`{"resultType":"matrix","result":[{"metric":{"alertstate":"firing"},"values":[%s]}]}`,
		vals)
	_, c := newAlertHygieneStub(t,
		[]string{"BrokenAlert"},
		map[string]string{"BrokenAlert": body},
	)
	a := New(Config{Lookback: 168 * time.Hour, Step: time.Hour, Now: func() time.Time { return time.Unix(1715000000, 0) }})
	res, err := a.Analyze(context.Background(), analyzers.Deps{Prom: c, Logger: slog.Default()})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(res.Findings) != 1 {
		t.Fatalf("Findings = %d, want 1", len(res.Findings))
	}
	f := res.Findings[0]
	if f.ID != "alert_hygiene/always_firing" {
		t.Errorf("ID = %q, want alert_hygiene/always_firing", f.ID)
	}
	if f.Severity != findings.SeverityCritical {
		t.Errorf("Severity = %v, want Critical", f.Severity)
	}
}

func TestAlertHygiene_PrefersVMAlertWhenSet(t *testing.T) {
	_, promC := newAlertHygieneStub(t, nil, nil) // Prom has no alerts
	_, vmC := newAlertHygieneStub(t,
		[]string{"VMAlert"},
		map[string]string{"VMAlert": `{"resultType":"matrix","result":[]}`},
	)
	a := New(Config{Lookback: 168 * time.Hour, Step: time.Hour, Now: func() time.Time { return time.Unix(1715000000, 0) }})
	res, err := a.Analyze(context.Background(), analyzers.Deps{Prom: promC, VMAlert: vmC, Logger: slog.Default()})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(res.Findings) != 1 || !strings.Contains(res.Findings[0].Title, "VMAlert") {
		t.Fatalf("expected VMAlert finding, got %+v", res.Findings)
	}
}

func TestAlertHygiene_RulesUnavailableWarning(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	t.Cleanup(srv.Close)
	c, err := prom.New(srv.URL, prom.WithFlavorHint(prom.FlavorProm))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	a := New(Config{Lookback: time.Hour, Step: time.Minute, Now: func() time.Time { return time.Unix(1715000000, 0) }})
	res, err := a.Analyze(context.Background(), analyzers.Deps{Prom: c, Logger: slog.Default()})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(res.Findings) != 0 {
		t.Errorf("Findings = %d, want 0", len(res.Findings))
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "rules unavailable") {
		t.Errorf("Warnings = %v, want rules-unavailable warning", res.Warnings)
	}
}

func TestAlertHygiene_VictoriaWithoutVMAlertWarns(t *testing.T) {
	mux := http.NewServeMux()
	// VM single-node-like buildinfo: has version but missing revision+goVersion,
	// which is how the flavor detector classifies VM in internal/prometheus/flavor.go.
	mux.HandleFunc("/api/v1/status/buildinfo", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"success","data":{"version":"v1.99.0"}}`))
	})
	// VM single-node returns 200 + empty groups for /api/v1/rules.
	mux.HandleFunc("/api/v1/rules", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"success","data":{"groups":[]}}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	// Intentionally no WithFlavorHint — exercise live detection.
	c, err := prom.New(srv.URL)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	a := New(Config{Lookback: time.Hour, Step: time.Minute, Now: func() time.Time { return time.Unix(1715000000, 0) }})
	res, err := a.Analyze(context.Background(), analyzers.Deps{Prom: c, Logger: slog.Default()})
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(res.Findings) != 0 {
		t.Errorf("Findings = %d, want 0", len(res.Findings))
	}
	if len(res.Warnings) != 1 || !strings.Contains(res.Warnings[0], "VictoriaMetrics detected without --vmalert") {
		t.Errorf("Warnings = %v, want VM-specific warning", res.Warnings)
	}
}
