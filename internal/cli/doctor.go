// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/remetric-dev/remetric/internal/config"
	"github.com/remetric-dev/remetric/internal/findings"
	graf "github.com/remetric-dev/remetric/internal/grafana"
	outjson "github.com/remetric-dev/remetric/internal/output/json"
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

doctor is a read-only diagnostic; it never modifies the target.

Use --output json to emit machine-readable output (e.g. for CI).`,
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

//nolint:gocyclo // straight-line config-gathering and best-effort enrichments; no branching to flatten
func runDoctor(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	cfg := configFrom(ctx)
	if cfg == nil || cfg.Prometheus.URL == "" {
		return &flagError{err: errors.New("--prometheus is required")}
	}
	if err := validateOutput(cfg.Output); err != nil {
		return &flagError{err: err}
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
		rep.Backend = client.Flavor().String()
		// The Prometheus minimum-version gate only applies to a Prometheus
		// backend; VictoriaMetrics ships an unrelated v1.x version line.
		if client.Flavor() == prom.FlavorVictoria {
			rep.VersionOK = true
		} else {
			rep.VersionOK = compareVersions(bi.Version, minPrometheusVersion) >= 0
			if !rep.VersionOK {
				rep.Errors = append(rep.Errors, findings.DoctorError{
					Check: "version",
					Error: fmt.Sprintf("prometheus %s is below minimum %s", bi.Version, minPrometheusVersion),
				})
			}
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
		rep.RuntimeInfoSupported = true
		rep.StorageRetention = rt.StorageRetention
	} else if errors.Is(err, prom.ErrNotSupported) {
		// Backend (e.g., VictoriaMetrics) doesn't expose runtimeinfo.
		// Leave RuntimeInfoSupported = false; renderer prints n/a.
		_ = err
	}
	if names, err := client.LabelValues(ctx, "__name__"); err == nil {
		rep.NumMetrics = int64(len(names))
	}

	rep.ElapsedMs = time.Since(start).Milliseconds()

	if err := renderDoctor(cfg, cmd.OutOrStdout(), rep); err != nil {
		return err
	}
	if len(rep.Errors) > 0 {
		return &exitError{code: 1, err: errors.New("doctor checks failed")}
	}
	return nil
}

// renderDoctor dispatches the DoctorReport to the configured output renderer.
func renderDoctor(cfg *config.Config, w io.Writer, rep findings.DoctorReport) error {
	switch cfg.Output {
	case "", "terminal":
		return terminal.New(w, terminal.WithColor(!cfg.NoColor)).RenderDoctor(rep)
	case "json":
		return outjson.New(w).RenderDoctor(rep)
	default:
		return &flagError{err: fmt.Errorf("unsupported --output %q (want terminal|json)", cfg.Output)}
	}
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
	switch strings.ToLower(cfg.Backend) {
	case "", "auto":
		// no hint — detection runs at first use
	case "prometheus":
		opts = append(opts, prom.WithFlavorHint(prom.FlavorProm))
	case "victoria":
		opts = append(opts, prom.WithFlavorHint(prom.FlavorVictoria))
	}
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

// buildGrafanaClient constructs a Grafana client from cfg. Returns
// (nil, nil) if cfg.Grafana.URL is empty (no Grafana configured).
func buildGrafanaClient(cfg *config.Config, ua string) (*graf.Client, error) {
	if cfg.Grafana.URL == "" {
		return nil, nil
	}
	opts := []graf.Option{graf.WithUserAgent(ua)}
	if cfg.Grafana.MaxInFlight > 0 {
		opts = append(opts, graf.WithMaxInFlight(cfg.Grafana.MaxInFlight))
	}
	if cfg.Grafana.Token != "" {
		opts = append(opts, graf.WithBearerToken(cfg.Grafana.Token))
	}
	if cfg.Grafana.BasicAuth != "" {
		parts := strings.SplitN(cfg.Grafana.BasicAuth, ":", 2)
		if len(parts) != 2 {
			return nil, errors.New("--grafana-basic-auth must be user:password")
		}
		opts = append(opts, graf.WithBasicAuth(parts[0], parts[1]))
	}
	if cfg.Grafana.TLSSkipVerify {
		opts = append(opts, graf.WithTLSSkipVerify(true))
	}
	if cfg.Timeout > 0 {
		opts = append(opts, graf.WithTimeout(cfg.Timeout))
	}
	return graf.New(cfg.Grafana.URL, opts...)
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
	s = strings.TrimPrefix(s, "v")
	var out [3]int
	parts := strings.Split(strings.SplitN(s, "-", 2)[0], ".")
	for i := 0; i < 3 && i < len(parts); i++ {
		out[i], _ = strconv.Atoi(parts[i])
	}
	return out
}
