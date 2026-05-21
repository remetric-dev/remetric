// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

//go:build e2e

package e2e

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestE2E_AlertsAlwaysFiring(t *testing.T) {
	waitForAlwaysFiringSamples(t, 12, 3*time.Minute)
	out, err := runCmd(t, binPath(t),
		"alerts", "always-firing",
		"--prometheus", "http://localhost:9090",
		"--lookback", "2m",
		"--step", "10s",
		"--output", "json",
	)
	if err != nil {
		t.Fatalf("alerts always-firing failed: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "RemetricAlwaysFiring") {
		t.Errorf("expected RemetricAlwaysFiring in JSON output:\n%s", out)
	}
}

func TestE2E_AlertsUnused(t *testing.T) {
	out, err := runCmd(t, binPath(t),
		"alerts", "unused",
		"--prometheus", "http://localhost:9090",
		"--lookback", "2m",
		"--step", "10s",
		"--output", "json",
	)
	if err != nil {
		t.Fatalf("alerts unused failed: %v\noutput:\n%s", err, out)
	}
	if !strings.Contains(out, "RemetricNeverFired") {
		t.Errorf("expected RemetricNeverFired in JSON output:\n%s", out)
	}
}

func TestE2E_ReportHTML(t *testing.T) {
	waitForAlwaysFiringSamples(t, 12, 3*time.Minute)
	tmp := t.TempDir()
	outFile := filepath.Join(tmp, "report.html")
	stdout, err := runCmd(t, binPath(t),
		"report",
		"--prometheus", "http://localhost:9090",
		"--format", "html",
		"--out", outFile,
		"--lookback", "2m",
		"--step", "10s",
		"--min-severity", "low",
	)
	if err != nil {
		t.Fatalf("report --format html failed: %v\nstdout:\n%s", err, stdout)
	}
	body, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read %s: %v", outFile, err)
	}
	bodyStr := string(body)
	if !strings.Contains(bodyStr, "<!DOCTYPE html>") {
		head := bodyStr
		if len(head) > 200 {
			head = head[:200]
		}
		t.Errorf("report missing DOCTYPE; head:\n%s", head)
	}
	if !strings.Contains(bodyStr, "RemetricAlwaysFiring") {
		t.Errorf("report missing RemetricAlwaysFiring finding")
	}
}

// waitForAlwaysFiringSamples polls Prometheus until count_over_time of the
// firing ALERTS series for RemetricAlwaysFiring reaches the required samples,
// or fails the test after the deadline. Eliminates a warm-up race where the
// stack has been up <2 min and the analyzer's 95% threshold can't yet trigger.
func waitForAlwaysFiringSamples(t *testing.T, minSamples int, deadline time.Duration) {
	t.Helper()
	query := `count_over_time(ALERTS{alertname="RemetricAlwaysFiring",alertstate="firing"}[2m])`
	endpoint := "http://localhost:9090/api/v1/query?query=" + url.QueryEscape(query)
	stop := time.Now().Add(deadline)
	for time.Now().Before(stop) {
		resp, err := http.Get(endpoint)
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			// Trivially parse the count from the response. We only need the
			// numeric value of the single sample.
			if n := parseInstantValue(body); n >= float64(minSamples) {
				return
			}
		}
		time.Sleep(2 * time.Second)
	}
	t.Fatalf("Prometheus did not accumulate %d firing samples within %s", minSamples, deadline)
}

// parseInstantValue extracts the numeric value from a Prom instant-vector query response.
// Returns 0 if the response shape is unexpected (treated as not-yet-ready).
func parseInstantValue(body []byte) float64 {
	// Response: {"status":"success","data":{"resultType":"vector","result":[{"metric":{...},"value":[<ts>,"<value>"]}]}}
	var env struct {
		Data struct {
			Result []struct {
				Value [2]any `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &env); err != nil || len(env.Data.Result) == 0 {
		return 0
	}
	s, ok := env.Data.Result[0].Value[1].(string)
	if !ok {
		return 0
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return v
}
