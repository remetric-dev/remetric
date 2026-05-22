// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package cli_test

import (
	"bytes"
	"encoding/json"
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

// TestAlertsUnused_IgnoreAlertScopedToClass pins the invariant that
// cfg.IgnoreFilter().Apply runs AFTER class-scoping in alerts.go, NOT
// before. A future refactor that moves the Apply call up to "be
// consistent with the focused commands" would silently double-count
// ignored findings across the two `alerts` subcommands. This test
// makes that regression LOUD.
//
// Stub: 1 never_fired (NoisyAlert) + 1 always_firing (BrokenAlert).
//
//   - `alerts unused --ignore-alert=NoisyAlert`     -> ignored_count == 1
//     (NoisyAlert is in the `unused` class; filter sees it)
//   - `alerts unused --ignore-alert=BrokenAlert`    -> ignored_count == 0
//     (BrokenAlert is always_firing, NOT in `unused` class;
//     class-scoping drops it before filter sees it)
func TestAlertsUnused_IgnoreAlertScopedToClass(t *testing.T) {
	vals := strings.Repeat(`[1715000000,"1"],`, 60)
	vals = strings.TrimSuffix(vals, ",")
	firingBody := fmt.Sprintf(
		`{"resultType":"matrix","result":[{"metric":{"alertstate":"firing"},"values":[%s]}]}`, vals)
	srv := newAlertsStub(t, map[string]string{
		"NoisyAlert":  `{"resultType":"matrix","result":[]}`,
		"BrokenAlert": firingBody,
	})

	parse := func(s string) int {
		t.Helper()
		var env struct {
			IgnoredCount int `json:"ignored_count"`
		}
		if err := json.Unmarshal([]byte(s), &env); err != nil {
			t.Fatalf("unmarshal: %v\nout: %s", err, s)
		}
		return env.IgnoredCount
	}

	// Case 1: --ignore-alert matches a finding IN this subcommand's class.
	{
		var stdout, stderr bytes.Buffer
		code := cli.ExecuteWith(cli.Args{
			Version: "test",
			Args: []string{
				"alerts", "unused", "--prometheus", srv.URL,
				"--lookback", "1h", "--step", "1m",
				"--min-severity", "low",
				"--output", "json",
				"--ignore-alert", "NoisyAlert",
			},
			Stdout: &stdout,
			Stderr: &stderr,
		})
		if code != 0 {
			t.Fatalf("case1 exit = %d, stderr=%s", code, stderr.String())
		}
		if got := parse(stdout.String()); got != 1 {
			t.Errorf("case1: ignored_count = %d, want 1", got)
		}
	}

	// Case 2: --ignore-alert matches a finding NOT in this subcommand's class.
	// If filter runs AFTER class-scoping (correct), ignored_count == 0.
	// If filter runs BEFORE class-scoping (regression), ignored_count == 1.
	{
		var stdout, stderr bytes.Buffer
		code := cli.ExecuteWith(cli.Args{
			Version: "test",
			Args: []string{
				"alerts", "unused", "--prometheus", srv.URL,
				"--lookback", "1h", "--step", "1m",
				"--min-severity", "low",
				"--output", "json",
				"--ignore-alert", "BrokenAlert",
			},
			Stdout: &stdout,
			Stderr: &stderr,
		})
		if code != 0 {
			t.Fatalf("case2 exit = %d, stderr=%s", code, stderr.String())
		}
		if got := parse(stdout.String()); got != 0 {
			t.Errorf("case2: ignored_count = %d, want 0 (BrokenAlert is always_firing, not in `unused` class)", got)
		}
	}
}
