// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

//go:build e2e

// VictoriaMetrics e2e tests. Require the VM stack at http://localhost:8428
// (and vmalert at :8880, cardinality-bomb-vm at :8081). Bring up with
// `make e2e-vm-up`. Skip cleanly if the stack isn't reachable.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os/exec"
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

// runCmdSplit runs the binary and returns stdout and stderr separately. The
// standalone `metrics unused` command writes graceful-degradation warnings to
// stderr (per T12), so tests that assert on those warnings need them split out
// from the JSON document on stdout.
func runCmdSplit(t *testing.T, exe string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var outBuf, errBuf bytes.Buffer
	cmd := exec.Command(exe, args...)
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

func TestE2E_VM_UnusedWithoutVMAlert(t *testing.T) {
	skipIfVMDown(t)
	time.Sleep(5 * time.Second)
	stdout, stderr, err := runCmdSplit(t, binPath(t),
		"metrics", "unused",
		"--prometheus", vmURL,
		"--output", "json",
		"--min-severity", "low",
	)
	if err != nil {
		t.Fatalf("metrics unused failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	// VictoriaMetrics single-node serves /api/v1/rules at HTTP 200 with empty
	// groups (not 404) because recording rules live in vmalert. Without
	// --vmalert we cannot distinguish "no rules configured" from "rules in
	// vmalert we can't see", so the analyzer surfaces the same
	// 'rules unavailable' graceful-degradation warning on stderr regardless
	// of payload, and app_requests_total (used by a vmalert recording rule
	// the analyzer can't see) shows up as unused.
	if !strings.Contains(stderr, "rules unavailable") {
		t.Errorf("expected 'rules unavailable' warning on stderr without --vmalert; stderr:\n%s", stderr)
	}
	trimmed := strings.TrimSpace(stdout)
	if !strings.HasPrefix(trimmed, "[") {
		// Empty-state or non-array document — skip the membership check.
		return
	}
	var fs []map[string]any
	if err := json.Unmarshal([]byte(trimmed), &fs); err != nil {
		t.Fatalf("parse findings JSON: %v\nstdout: %s\nstderr: %s", err, stdout, stderr)
	}
	var sawAppRequests bool
	for _, f := range fs {
		metric, _ := f["metric"].(string)
		if metric == "app_requests_total" {
			sawAppRequests = true
			break
		}
	}
	if !sawAppRequests {
		t.Errorf("expected app_requests_total in unused findings without --vmalert; findings:\n%s", stdout)
	}
}

func TestE2E_VM_UnusedWithVMAlert(t *testing.T) {
	skipIfVMDown(t)
	time.Sleep(5 * time.Second)
	stdout, stderr, err := runCmdSplit(t, binPath(t),
		"metrics", "unused",
		"--prometheus", vmURL,
		"--vmalert", vmalertURL,
		"--output", "json",
		"--min-severity", "low",
	)
	if err != nil {
		t.Fatalf("metrics unused failed: %v\nstdout:\n%s\nstderr:\n%s", err, stdout, stderr)
	}
	if strings.Contains(stderr, "rules unavailable") {
		t.Errorf("did not expect 'rules unavailable' with --vmalert set:\nstderr:\n%s", stderr)
	}
	// Stdout is a JSON array of findings, or the empty-state document. Tolerate
	// both: parse as a generic value and only inspect when it's a slice.
	trimmed := strings.TrimSpace(stdout)
	if strings.HasPrefix(trimmed, "[") {
		var fs []map[string]any
		if err := json.Unmarshal([]byte(trimmed), &fs); err != nil {
			t.Fatalf("parse findings JSON: %v\nstdout: %s", err, stdout)
		}
		for _, f := range fs {
			metric, _ := f["metric"].(string)
			if metric == "app_requests_total" {
				t.Errorf("app_requests_total appeared in unused findings despite being used in recording rule: %v", f)
			}
		}
	}
}

func TestE2E_VM_Scan(t *testing.T) {
	skipIfVMDown(t)
	time.Sleep(5 * time.Second)
	out, err := runCmd(t, binPath(t), "scan",
		"--prometheus", vmURL,
		"--vmalert", vmalertURL,
		"--output", "json",
		"--min-severity", "low",
	)
	if err != nil {
		t.Fatalf("scan failed: %v\noutput:\n%s", err, out)
	}
	var report struct {
		Findings []map[string]any `json:"findings"`
		Warnings []string         `json:"warnings"`
	}
	if err := json.Unmarshal([]byte(out), &report); err != nil {
		t.Fatalf("parse scan JSON: %v\nout: %s", err, out)
	}
	if len(report.Findings) == 0 {
		t.Errorf("expected non-empty findings from full scan against VM stack; out:\n%s", out)
	}
}
