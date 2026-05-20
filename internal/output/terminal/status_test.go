package terminal

import (
	"bytes"
	"errors"
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
