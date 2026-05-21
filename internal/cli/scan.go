// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package cli

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/remetric-dev/remetric/internal/analyzers"
	"github.com/remetric-dev/remetric/internal/analyzers/alerthygiene"
	"github.com/remetric-dev/remetric/internal/analyzers/cardinality"
	"github.com/remetric-dev/remetric/internal/analyzers/labelpattern"
	"github.com/remetric-dev/remetric/internal/analyzers/unusedmetrics"
	"github.com/remetric-dev/remetric/internal/config"
	"github.com/remetric-dev/remetric/internal/findings"
	prom "github.com/remetric-dev/remetric/internal/prometheus"
)

func newScanCmd() *cobra.Command {
	var minSeverity string
	cmd := &cobra.Command{
		Use:   "scan",
		Short: "Run every available analyzer and emit a unified report.",
		Long: `Orchestrates cardinality, label-pattern, alert-hygiene, and
(if --grafana is set) unused-metrics analyzers. Findings are merged
into a single findings.Report shape — see the JSON schema in the spec.`,
		Example: `  # Full scan with Grafana
  remetric scan --prometheus http://localhost:9090 --grafana http://localhost:3000

  # JSON for CI
  remetric scan --prometheus http://localhost:9090 --output json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cfg := configFrom(ctx)
			if cfg == nil || cfg.Prometheus.URL == "" {
				return &flagError{err: errors.New("--prometheus is required")}
			}
			if err := validateOutput(cfg.Output); err != nil {
				return &flagError{err: err}
			}
			minSev, err := findings.ParseSeverity(minSeverity)
			if err != nil {
				return &flagError{err: fmt.Errorf("invalid --min-severity: %w", err)}
			}
			if cfg.Timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
				defer cancel()
			}

			promClient, err := buildPromClient(cfg, "remetric/scan")
			if err != nil {
				return &exitError{code: 2, err: err}
			}
			grafClient, err := buildGrafanaClient(cfg, "remetric/scan")
			if err != nil {
				return &exitError{code: 2, err: err}
			}
			vmalertClient, err := buildVMAlertClient(cfg, "remetric/scan")
			if err != nil {
				return &exitError{code: 2, err: err}
			}

			deps := analyzers.Deps{
				Prom:    promClient,
				Graf:    grafClient,
				VMAlert: vmalertClient,
				Logger:  loggerFrom(ctx),
				Limits:  analyzers.DefaultLimits(),
			}

			var (
				all      []findings.Finding
				warnings []string
			)
			runners := []analyzers.Analyzer{
				cardinality.New(),
				labelpattern.New(),
				unusedmetrics.New(),
				alerthygiene.New(alerthygiene.Config{}),
			}
			for _, a := range runners {
				res, err := a.Analyze(ctx, deps)
				if err != nil {
					return &exitError{code: 1, err: fmt.Errorf("%s: %w", a.Name(), err)}
				}
				all = append(all, res.Findings...)
				warnings = append(warnings, res.Warnings...)
			}
			filtered := filterAtLeast(all, minSev)
			sort.SliceStable(filtered, func(i, j int) bool {
				if filtered[i].Severity != filtered[j].Severity {
					return filtered[i].Severity > filtered[j].Severity
				}
				return filtered[i].Evidence.SeriesCount > filtered[j].Evidence.SeriesCount
			})

			rep := buildReport(cmd, cfg, filtered, promClient)
			rep.Warnings = warnings
			rep.ScannedAt = time.Now().UTC()
			return renderReport(cfg, cmd.OutOrStdout(), rep)
		},
	}
	cmd.Flags().StringVar(&minSeverity, "min-severity", "medium", "Minimum severity to print: low|medium|high|critical")
	return cmd
}

// buildReport assembles a Report with Target + Overview populated
// from a fresh Prom call. Errors fetching metadata degrade silently —
// the report renders without those fields rather than failing the scan.
func buildReport(cmd *cobra.Command, cfg *config.Config, fs []findings.Finding, promClient *prom.Client) *findings.Report {
	rep := findings.NewReport(cmd.Root().Version, fs)
	rep.Target = &findings.Target{PrometheusURL: cfg.Prometheus.URL}
	ctx := cmd.Context()
	if bi, err := promClient.BuildInfo(ctx); err == nil {
		rep.Target.PrometheusVersion = bi.Version
	}
	if names, err := promClient.LabelValues(ctx, "__name__"); err == nil {
		rep.Overview = &findings.Overview{TotalMetrics: int64(len(names))}
	}
	if stats, err := promClient.TSDBStats(ctx, 1); err == nil {
		if rep.Overview == nil {
			rep.Overview = &findings.Overview{}
		}
		rep.Overview.TotalSeries = stats.HeadStats.NumSeries
	}
	return rep
}
