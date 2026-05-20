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
	if !strings.Contains(out, "remetric") && !strings.Contains(out, "Severity") && !strings.Contains(out, "No cardinality findings") {
		t.Errorf("unexpected output:\n%s", out)
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
