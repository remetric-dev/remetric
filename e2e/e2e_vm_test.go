// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

//go:build e2e

// VictoriaMetrics e2e tests. Require the VM stack at http://localhost:8428
// (and vmalert at :8880, cardinality-bomb-vm at :8081). Bring up with
// `make e2e-vm-up`. Skip cleanly if the stack isn't reachable.
package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"
)

const (
	vmURL      = "http://localhost:8428"
	vmalertURL = "http://localhost:8880"
)

func skipIfVMDown(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, vmURL+"/-/healthy", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Skip("VM stack not up at " + vmURL + "; run `make e2e-vm-up`")
	}
	_ = resp.Body.Close()
}

func TestE2E_VM_Doctor(t *testing.T) {
	skipIfVMDown(t)
	out, err := runCmd(t, binPath(t), "doctor", "--prometheus", vmURL, "--no-color")
	if err != nil {
		t.Fatalf("doctor failed: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(strings.ToLower(out), "victoria") {
		t.Errorf("expected 'victoria' backend label in doctor output:\n%s", out)
	}
	if !strings.Contains(out, "n/a") {
		t.Errorf("expected 'n/a' for retention on VM:\n%s", out)
	}
}

func TestE2E_VM_CardinalityTop(t *testing.T) {
	skipIfVMDown(t)
	// Give VM a moment to ingest cardinality-bomb metrics.
	time.Sleep(5 * time.Second)
	out, err := runCmd(t, binPath(t), "cardinality", "top",
		"--prometheus", vmURL,
		"--output", "json",
		"--min-severity", "low",
	)
	if err != nil {
		t.Fatalf("cardinality top failed: %v\noutput:\n%s", err, out)
	}
	var report struct {
		Findings []map[string]any `json:"findings"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("parse JSON: %v\nout: %s", err, out)
	}
	if len(report.Findings) == 0 {
		t.Errorf("expected at least one cardinality finding from cardinality-bomb-vm, got 0; output:\n%s", out)
	}
}

func TestE2E_VM_CardinalitySuspicious(t *testing.T) {
	skipIfVMDown(t)
	time.Sleep(5 * time.Second)
	out, err := runCmd(t, binPath(t), "cardinality", "suspicious",
		"--prometheus", vmURL,
		"--output", "json",
		"--min-severity", "low",
	)
	if err != nil {
		t.Fatalf("cardinality suspicious failed: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "user_id") && !strings.Contains(out, "trace_id") {
		t.Errorf("expected user_id or trace_id in suspicious findings:\n%s", out)
	}
}
