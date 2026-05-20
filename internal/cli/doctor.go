// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package cli

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/remetric-dev/remetric/internal/config"
	"github.com/remetric-dev/remetric/internal/findings"
	"github.com/remetric-dev/remetric/internal/output/terminal"
	prom "github.com/remetric-dev/remetric/internal/prometheus"
)

const minPrometheusVersion = "2.30.0"

func newDoctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check Prometheus connectivity, version, and permissions.",
		Long: `Verifies that the configured Prometheus is reachable, advertises a
supported version (2.30+), exposes the TSDB stats endpoint, and shows
the head series count, total metric names, and configured retention.

doctor is a read-only diagnostic; it never modifies the target.`,
		Example: `  # Default checks (no auth)
  remetric doctor --prometheus http://localhost:9090

  # Bearer token
  remetric doctor --prometheus https://prom.example.com --prom-token $TOKEN

  # Basic auth
  remetric doctor --prometheus https://prom.example.com --prom-basic-auth user:pass

  # Skip self-signed TLS check (use with care)
  remetric doctor --prometheus https://prom.example.com --prom-tls-skip-verify`,
		RunE: runDoctor,
	}
}

func runDoctor(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	cfg := configFrom(ctx)
	if cfg == nil || cfg.Prometheus.URL == "" {
		return &flagError{err: errors.New("--prometheus is required")}
	}

	if cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
		defer cancel()
	}

	start := time.Now()
	client, err := buildPromClient(cfg, "remetric/doctor")
	if err != nil {
		return &exitError{code: 2, err: err}
	}

	rep := findings.DoctorReport{
		PrometheusURL: cfg.Prometheus.URL,
		AuthMethod:    authMethodFor(cfg),
	}

	if err := client.Ping(ctx); err != nil {
		rep.Errors = append(rep.Errors, findings.DoctorError{Check: "reachability", Error: err.Error()})
	} else {
		rep.Reachable = true
	}

	if bi, err := client.BuildInfo(ctx); err != nil {
		rep.Errors = append(rep.Errors, findings.DoctorError{Check: "buildinfo", Error: err.Error()})
	} else {
		rep.Version = bi.Version
		rep.VersionOK = compareVersions(bi.Version, minPrometheusVersion) >= 0
		if !rep.VersionOK {
			rep.Errors = append(rep.Errors, findings.DoctorError{
				Check: "version",
				Error: fmt.Sprintf("prometheus %s is below minimum %s", bi.Version, minPrometheusVersion),
			})
		}
	}

	if stats, err := client.TSDBStats(ctx, 1); err != nil {
		rep.Errors = append(rep.Errors, findings.DoctorError{Check: "tsdb_stats", Error: err.Error()})
	} else {
		rep.TSDBStatsOK = true
		rep.NumSeries = stats.HeadStats.NumSeries
	}

	// Best-effort enrichments: failures are silently ignored so the report
	// simply omits these fields. The [OK] markers above remain authoritative.
	if rt, err := client.RuntimeInfo(ctx); err == nil {
		rep.StorageRetention = rt.StorageRetention
	}
	if names, err := client.LabelValues(ctx, "__name__"); err == nil {
		rep.NumMetrics = int64(len(names))
	}

	rep.ElapsedMs = time.Since(start).Milliseconds()

	r := terminal.New(cmd.OutOrStdout(), terminal.WithColor(!cfg.NoColor))
	if rerr := r.RenderDoctor(rep); rerr != nil {
		return rerr
	}
	if len(rep.Errors) > 0 {
		return &exitError{code: 1, err: errors.New("doctor checks failed")}
	}
	return nil
}

func authMethodFor(cfg *config.Config) string {
	switch {
	case cfg.Prometheus.Token != "":
		return "bearer"
	case cfg.Prometheus.BasicAuth != "":
		return "basic"
	default:
		return "none"
	}
}

func buildPromClient(cfg *config.Config, ua string) (*prom.Client, error) {
	opts := []prom.Option{prom.WithUserAgent(ua), prom.WithMaxInFlight(cfg.Prometheus.MaxInFlight)}
	if cfg.Prometheus.Token != "" {
		opts = append(opts, prom.WithBearerToken(cfg.Prometheus.Token))
	}
	if cfg.Prometheus.BasicAuth != "" {
		parts := strings.SplitN(cfg.Prometheus.BasicAuth, ":", 2)
		if len(parts) != 2 {
			return nil, errors.New("--prom-basic-auth must be user:password")
		}
		opts = append(opts, prom.WithBasicAuth(parts[0], parts[1]))
	}
	if cfg.Prometheus.TLSSkipVerify {
		opts = append(opts, prom.WithTLSSkipVerify(true))
	}
	if cfg.Timeout > 0 {
		opts = append(opts, prom.WithTimeout(cfg.Timeout))
	}
	return prom.New(cfg.Prometheus.URL, opts...)
}

// compareVersions compares dotted semver-ish strings ignoring pre-release.
// Returns -1, 0, +1.
func compareVersions(a, b string) int {
	pa := versionParts(a)
	pb := versionParts(b)
	for i := 0; i < 3; i++ {
		if pa[i] < pb[i] {
			return -1
		}
		if pa[i] > pb[i] {
			return +1
		}
	}
	return 0
}

func versionParts(s string) [3]int {
	var out [3]int
	parts := strings.Split(strings.SplitN(s, "-", 2)[0], ".")
	for i := 0; i < 3 && i < len(parts); i++ {
		out[i], _ = strconv.Atoi(parts[i])
	}
	return out
}
