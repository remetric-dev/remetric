// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package json

import (
	"bytes"
	"strings"
	"testing"

	"github.com/remetric-dev/remetric/internal/findings"
)

func TestRenderDoctor_Golden(t *testing.T) {
	rep := findings.DoctorReport{
		PrometheusURL:    "http://prom:9090",
		Reachable:        true,
		Version:          "2.51.2",
		VersionOK:        true,
		AuthMethod:       "none",
		TSDBStatsOK:      true,
		NumSeries:        1_234_567,
		NumMetrics:       4_127,
		StorageRetention: "15d",
		ElapsedMs:        27,
	}
	var buf bytes.Buffer
	if err := New(&buf).RenderDoctor(rep); err != nil {
		t.Fatalf("RenderDoctor: %v", err)
	}
	checkGolden(t, "doctor.json", buf.Bytes())
}

func TestRenderDoctor_WithErrors(t *testing.T) {
	rep := findings.DoctorReport{
		PrometheusURL: "http://prom:9090",
		Reachable:     false,
		ElapsedMs:     5,
		Errors: []findings.DoctorError{
			{Check: "reachability", Error: "connection refused"},
		},
	}
	var buf bytes.Buffer
	if err := New(&buf).RenderDoctor(rep); err != nil {
		t.Fatalf("RenderDoctor: %v", err)
	}
	got := buf.String()
	for _, want := range []string{
		`"reachable": false`,
		`"errors"`,
		`"check": "reachability"`,
		`"error": "connection refused"`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q in output:\n%s", want, got)
		}
	}
}
