// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package cli

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"github.com/spf13/cobra"

	"github.com/remetric-dev/remetric/internal/analyzers"
	"github.com/remetric-dev/remetric/internal/analyzers/unusedmetrics"
	"github.com/remetric-dev/remetric/internal/findings"
)

func newMetricsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "Inspect metric-level findings (unused metrics).",
		Long: `Metrics subcommands compare what Prometheus ingests against what
Grafana / alert rules / recording rules actually reference.

Subcommands:
  unused  Flag metric names with no observed consumer.`,
	}
	cmd.AddCommand(newMetricsUnusedCmd())
	return cmd
}

//nolint:gocyclo // RunE closure is straight-line config-gathering; no branching to flatten
func newMetricsUnusedCmd() *cobra.Command {
	var (
		limit       int
		minSeverity string
	)
	cmd := &cobra.Command{
		Use:   "unused",
		Short: "List ingested metrics with no Grafana / alert / recording-rule reference.",
		Long: `Compares the set of ingested metric names with the union of
metrics referenced by Grafana dashboards, alert rule expressions,
and recording rule expressions (plus the recording rule output
names themselves).

Without --grafana, dashboards are not consulted; only alert and
recording rule references count toward "used".`,
		Example: `  # Diff against Prometheus rules only
  remetric metrics unused --prometheus http://localhost:9090

  # Include Grafana dashboards
  remetric metrics unused --prometheus http://localhost:9090 --grafana http://localhost:3000

  # JSON output
  remetric metrics unused --prometheus http://localhost:9090 --output json`,
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

			prom, err := buildPromClient(cfg, "remetric/metrics-unused")
			if err != nil {
				return &exitError{code: 2, err: err}
			}
			graf, err := buildGrafanaClient(cfg, "remetric/metrics-unused")
			if err != nil {
				return &exitError{code: 2, err: err}
			}
			vmalert, err := buildVMAlertClient(cfg, "remetric/metrics-unused")
			if err != nil {
				return &exitError{code: 2, err: err}
			}

			d := analyzers.Deps{
				Prom:    prom,
				Graf:    graf,
				VMAlert: vmalert,
				Logger:  loggerFrom(ctx),
				Limits:  analyzers.DefaultLimits(),
			}
			res, err := unusedmetrics.New().Analyze(ctx, d)
			if err != nil {
				return &exitError{code: 1, err: err}
			}
			for _, w := range res.Warnings {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "! warning: %s\n", w)
			}
			all := res.Findings

			filtered := filterAtLeast(all, minSev)
			if len(filtered) == 0 {
				return renderEmpty(cfg, cmd.OutOrStdout(), unusedMetricsCopy, minSev, len(all), tallyBySeverity(all))
			}

			sort.SliceStable(filtered, func(i, j int) bool {
				if filtered[i].Severity != filtered[j].Severity {
					return filtered[i].Severity > filtered[j].Severity
				}
				return filtered[i].Evidence.SeriesCount > filtered[j].Evidence.SeriesCount
			})
			if limit >= 0 && len(filtered) > limit {
				filtered = filtered[:limit]
			}
			if len(filtered) == 0 {
				return renderEmpty(cfg, cmd.OutOrStdout(), unusedMetricsCopy, minSev, 0, nil)
			}

			if err := renderFindings(cfg, cmd.OutOrStdout(), filtered, 0); err != nil {
				return err
			}
			sev, enabled := cfg.FailOnThreshold()
			if findings.ShouldFail(sev, enabled, filtered) {
				return &exitError{code: 3, err: fmt.Errorf("findings at or above --fail-on=%s", cfg.FailOn)}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum findings to print (0 = none)")
	cmd.Flags().StringVar(&minSeverity, "min-severity", "medium", "Minimum severity to print: low|medium|high|critical")
	return cmd
}
