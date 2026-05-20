// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

//go:build e2e

// Package e2e contains end-to-end smoke tests requiring a live Prometheus
// at http://localhost:9090. Run `make e2e-up` first.
package e2e

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func binPath(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	repo := filepath.Join(filepath.Dir(file), "..")
	return filepath.Join(repo, "bin", "remetric")
}

func TestE2E_Doctor(t *testing.T) {
	out, err := runCmd(t, binPath(t), "doctor", "--prometheus", "http://localhost:9090", "--no-color")
	if err != nil {
		t.Fatalf("doctor failed: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "[OK]") {
		t.Errorf("expected [OK] in output:\n%s", out)
	}
}

func TestE2E_CardinalityTop(t *testing.T) {
	out, err := runCmd(t, binPath(t), "cardinality", "top", "--prometheus", "http://localhost:9090", "--no-color", "--min-severity", "low")
	if err != nil {
		t.Fatalf("cardinality top failed: %v\noutput:\n%s", err, out)
	}
	upper := strings.ToUpper(out)
	// Either we got a populated table (header contains "SEVERITY") or the empty-state sentence.
	if !strings.Contains(upper, "SEVERITY") && !strings.Contains(out, "No cardinality findings") {
		t.Errorf("unexpected output (no table header, no empty sentence):\n%s", out)
	}
}

func TestE2E_CardinalitySuspicious_JSON(t *testing.T) {
	out, err := runCmd(t, binPath(t), "cardinality", "suspicious", "--prometheus", "http://localhost:9090", "--output", "json", "--min-severity", "low")
	if err != nil {
		t.Fatalf("cardinality suspicious failed: %v\noutput:\n%s", err, out)
	}
	if !strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Errorf("expected JSON document, got:\n%s", out)
	}
	if !strings.Contains(out, `"findings"`) {
		t.Errorf("expected findings key, got:\n%s", out)
	}
}

func TestE2E_CardinalityLabels_JSON(t *testing.T) {
	out, err := runCmd(t, binPath(t), "cardinality", "labels", "--metric", "up", "--prometheus", "http://localhost:9090", "--output", "json")
	if err != nil {
		t.Fatalf("cardinality labels failed: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, `"metric": "up"`) {
		t.Errorf("expected metric key with 'up' value, got:\n%s", out)
	}
	if !strings.Contains(out, `"labels"`) {
		t.Errorf("expected labels key, got:\n%s", out)
	}
}

func TestE2E_UnusedMetrics_Terminal(t *testing.T) {
	out, err := runCmd(t, binPath(t),
		"unused", "metrics",
		"--prometheus", "http://localhost:9090",
		"--grafana", "http://localhost:3000",
		"--no-color",
		"--min-severity", "low",
	)
	if err != nil {
		t.Fatalf("unused metrics failed: %v\n%s", err, out)
	}
	upper := strings.ToUpper(out)
	if !strings.Contains(upper, "SEVERITY") && !strings.Contains(out, "No unused metrics") {
		t.Errorf("unexpected output (no table header, no empty sentence):\n%s", out)
	}
}

func TestE2E_UnusedMetrics_JSON(t *testing.T) {
	out, err := runCmd(t, binPath(t),
		"unused", "metrics",
		"--prometheus", "http://localhost:9090",
		"--grafana", "http://localhost:3000",
		"--output", "json",
		"--min-severity", "low",
	)
	if err != nil {
		t.Fatalf("unused metrics --output json failed: %v\n%s", err, out)
	}
	if !strings.HasPrefix(strings.TrimSpace(out), "{") {
		t.Errorf("expected JSON document, got:\n%s", out)
	}
	if !strings.Contains(out, `"findings"`) {
		t.Errorf("expected findings key, got:\n%s", out)
	}
}

func TestE2E_Scan_Report(t *testing.T) {
	out, err := runCmd(t, binPath(t),
		"scan",
		"--prometheus", "http://localhost:9090",
		"--grafana", "http://localhost:3000",
		"--output", "json",
		"--min-severity", "low",
	)
	if err != nil {
		t.Fatalf("scan failed: %v\n%s", err, out)
	}
	for _, want := range []string{`"findings"`, `"target"`, `"summary"`} {
		if !strings.Contains(out, want) {
			t.Errorf("scan JSON missing %q:\n%s", want, out)
		}
	}
}

func runCmd(t *testing.T, name string, args ...string) (string, error) {
	t.Helper()
	var buf bytes.Buffer
	cmd := exec.Command(name, args...)
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}
