// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package cli_test

import (
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/remetric-dev/remetric/internal/cli"
)

func newAlertsStub(t *testing.T, alertResults map[string]string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/rules", func(w http.ResponseWriter, _ *http.Request) {
		var b strings.Builder
		i := 0
		for name := range alertResults {
			if i > 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(&b, `{"name":%q,"query":"up","type":"alerting"}`, name)
			i++
		}
		fmt.Fprintf(w, `{"status":"success","data":{"groups":[{"name":"g","file":"f.yml","rules":[%s]}]}}`, b.String())
	})
	mux.HandleFunc("/api/v1/query_range", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("query")
		name := ""
		if i := strings.Index(q, `alertname="`); i >= 0 {
			rest := q[i+len(`alertname="`):]
			if j := strings.Index(rest, `"`); j >= 0 {
				name = rest[:j]
			}
		}
		body, ok := alertResults[name]
		if !ok {
			body = `{"resultType":"matrix","result":[]}`
		}
		fmt.Fprintf(w, `{"status":"success","data":%s}`, body)
	})
	mux.HandleFunc("/api/v1/status/buildinfo", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"status":"success","data":{"version":"2.51.2","revision":"abc","goVersion":"go1.22"}}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func TestAlertsUnused_FlagsNeverFired(t *testing.T) {
	srv := newAlertsStub(t, map[string]string{
		"NoisyAlert": `{"resultType":"matrix","result":[]}`,
	})
	var stdout, stderr bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args:    []string{"alerts", "unused", "--prometheus", srv.URL, "--lookback", "1h", "--step", "1m", "--output", "json"},
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	if code != 0 {
		t.Fatalf("exit code = %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "NoisyAlert") {
		t.Errorf("stdout missing NoisyAlert:\n%s", stdout.String())
	}
}

func TestAlertsAlwaysFiring_FlagsAlwaysFiring(t *testing.T) {
	vals := strings.Repeat(`[1715000000,"1"],`, 60)
	vals = strings.TrimSuffix(vals, ",")
	body := fmt.Sprintf(
		`{"resultType":"matrix","result":[{"metric":{"alertstate":"firing"},"values":[%s]}]}`, vals)
	srv := newAlertsStub(t, map[string]string{"BrokenAlert": body})

	var stdout, stderr bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args:    []string{"alerts", "always-firing", "--prometheus", srv.URL, "--lookback", "1h", "--step", "1m", "--output", "json"},
		Stdout:  &stdout,
		Stderr:  &stderr,
	})
	if code != 0 {
		t.Fatalf("exit code = %d, stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "BrokenAlert") {
		t.Errorf("stdout missing BrokenAlert:\n%s", stdout.String())
	}
}

func TestAlertsUnused_EmptyStateScopesToClass(t *testing.T) {
	// Build a stub with one always_firing alert (60 firing samples at step=1m, lookback=1h = 100%)
	// and zero never_fired alerts. `alerts unused` should report "no never_fired alerts" without
	// citing the always_firing finding in the severity hint.
	vals := strings.Repeat(`[1715000000,"1"],`, 60)
	vals = strings.TrimSuffix(vals, ",")
	body := fmt.Sprintf(
		`{"resultType":"matrix","result":[{"metric":{"alertstate":"firing"},"values":[%s]}]}`, vals)
	srv := newAlertsStub(t, map[string]string{"AlwaysOn": body})

	var stdout, stderr bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args: []string{
			"alerts", "unused", "--prometheus", srv.URL,
			"--lookback", "1h", "--step", "1m",
			"--min-severity", "low",
		},
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("exit code = %d, stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	// Should be the no-results copy, NOT the "Filtered N unused alerts below ... critical" hint.
	if !strings.Contains(out, "No unused alerts detected") {
		t.Errorf("stdout missing no-results copy:\n%s", out)
	}
	// Sanity: the always_firing alert name must NOT appear in unused output.
	if strings.Contains(out, "AlwaysOn") {
		t.Errorf("unused output leaked always_firing finding:\n%s", out)
	}
}

// alert_hygiene/never_fired is hard-coded to SeverityMedium, so the gate
// for `alerts unused` is exercised with --fail-on medium rather than critical.
func TestAlertsUnused_FailOnMediumExits3WhenMediumFindingPresent(t *testing.T) {
	srv := newAlertsStub(t, map[string]string{
		"NoisyAlert": `{"resultType":"matrix","result":[]}`,
	})
	var stdout, stderr bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args: []string{
			"alerts", "unused",
			"--prometheus", srv.URL,
			"--lookback", "1h", "--step", "1m",
			"--fail-on", "medium",
		},
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 3 {
		t.Fatalf("exit code = %d, want 3 (stderr=%s)", code, stderr.String())
	}
}

func TestAlertsUnused_FailOnNoneExits0EvenWithMedium(t *testing.T) {
	srv := newAlertsStub(t, map[string]string{
		"NoisyAlert": `{"resultType":"matrix","result":[]}`,
	})
	var stdout, stderr bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args: []string{
			"alerts", "unused",
			"--prometheus", srv.URL,
			"--lookback", "1h", "--step", "1m",
			"--fail-on", "none",
		},
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr=%s)", code, stderr.String())
	}
}

func TestAlertsAlwaysFiring_FailOnCriticalExits3WhenCriticalFindingPresent(t *testing.T) {
	vals := strings.Repeat(`[1715000000,"1"],`, 60)
	vals = strings.TrimSuffix(vals, ",")
	body := fmt.Sprintf(
		`{"resultType":"matrix","result":[{"metric":{"alertstate":"firing"},"values":[%s]}]}`, vals)
	srv := newAlertsStub(t, map[string]string{"BrokenAlert": body})

	var stdout, stderr bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args: []string{
			"alerts", "always-firing",
			"--prometheus", srv.URL,
			"--lookback", "1h", "--step", "1m",
			"--fail-on", "critical",
		},
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 3 {
		t.Fatalf("exit code = %d, want 3 (stderr=%s)", code, stderr.String())
	}
}

func TestAlertsAlwaysFiring_FailOnNoneExits0EvenWithCritical(t *testing.T) {
	vals := strings.Repeat(`[1715000000,"1"],`, 60)
	vals = strings.TrimSuffix(vals, ",")
	body := fmt.Sprintf(
		`{"resultType":"matrix","result":[{"metric":{"alertstate":"firing"},"values":[%s]}]}`, vals)
	srv := newAlertsStub(t, map[string]string{"BrokenAlert": body})

	var stdout, stderr bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args: []string{
			"alerts", "always-firing",
			"--prometheus", srv.URL,
			"--lookback", "1h", "--step", "1m",
			"--fail-on", "none",
		},
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 (stderr=%s)", code, stderr.String())
	}
}

func TestAlertsUnused_LimitZeroShowsNoResults(t *testing.T) {
	// Even though there's a matching never_fired finding, --limit 0 should suppress it and
	// render the "no results" copy, not the severity-hint message.
	srv := newAlertsStub(t, map[string]string{
		"NoisyAlert": `{"resultType":"matrix","result":[]}`,
	})
	var stdout, stderr bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args: []string{
			"alerts", "unused", "--prometheus", srv.URL,
			"--lookback", "1h", "--step", "1m",
			"--min-severity", "low",
			"--limit", "0",
		},
		Stdout: &stdout,
		Stderr: &stderr,
	})
	if code != 0 {
		t.Fatalf("exit code = %d, stderr=%s", code, stderr.String())
	}
	out := stdout.String()
	if !strings.Contains(out, "No unused alerts detected") {
		t.Errorf("expected no-results copy with --limit 0; got:\n%s", out)
	}
}
