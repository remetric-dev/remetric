// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package terminal

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestRenderDoctor_Healthy(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, WithColor(false), WithWidth(100))
	rep := DoctorReport{
		PrometheusURL: "http://localhost:9090",
		Reachable:     true,
		Version:       "2.51.2",
		VersionOK:     true,
		AuthMethod:    "none",
		TSDBStatsOK:   true,
		NumSeries:     2341892,
		Elapsed:       412 * time.Millisecond,
	}
	if err := r.RenderDoctor(rep); err != nil {
		t.Fatalf("err = %v", err)
	}
	goldenAssert(t, "doctor_healthy.golden", buf.String())
}

func TestRenderDoctor_OldVersion(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, WithColor(false), WithWidth(100))
	rep := DoctorReport{
		PrometheusURL: "http://localhost:9090",
		Reachable:     true,
		Version:       "2.29.1",
		VersionOK:     false,
		AuthMethod:    "bearer",
		TSDBStatsOK:   false,
		Errors:        []DoctorError{{Check: "version", Err: errors.New("prometheus 2.29.1 is below minimum 2.30.0")}},
		Elapsed:       300 * time.Millisecond,
	}
	if err := r.RenderDoctor(rep); err != nil {
		t.Fatalf("err = %v", err)
	}
	goldenAssert(t, "doctor_old_version.golden", buf.String())
}

func TestRenderDoctor_IncludesMetricsAndRetention(t *testing.T) {
	rep := DoctorReport{
		PrometheusURL:    "http://prom:9090",
		Reachable:        true,
		Version:          "2.51.2",
		VersionOK:        true,
		AuthMethod:       "none",
		TSDBStatsOK:      true,
		NumSeries:        1_234_567,
		NumMetrics:       4_127,
		StorageRetention: "15d",
		Elapsed:          time.Second,
	}
	var buf bytes.Buffer
	r := New(&buf, WithColor(false))
	if err := r.RenderDoctor(rep); err != nil {
		t.Fatalf("RenderDoctor: %v", err)
	}
	body := buf.String()
	for _, want := range []string{
		"active series: 1,234,567",
		"metric names: 4,127",
		"retention:    15d",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("expected %q in output:\n%s", want, body)
		}
	}
}

func TestRenderDoctor_OmitsZeroExtras(t *testing.T) {
	rep := DoctorReport{
		PrometheusURL: "http://prom:9090",
		Reachable:     true,
		TSDBStatsOK:   true,
		// NumMetrics, StorageRetention intentionally zero/empty.
	}
	var buf bytes.Buffer
	r := New(&buf, WithColor(false))
	if err := r.RenderDoctor(rep); err != nil {
		t.Fatalf("RenderDoctor: %v", err)
	}
	body := buf.String()
	if strings.Contains(body, "metric names:") {
		t.Errorf("expected metric names line to be omitted, got:\n%s", body)
	}
	if strings.Contains(body, "retention:") {
		t.Errorf("expected retention line to be omitted, got:\n%s", body)
	}
}
