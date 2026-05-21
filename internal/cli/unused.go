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

func newUnusedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "unused",
		Short: "Find unused parts of the stack (metrics, dashboards, alerts).",
		Long: `Unused subcommands compare what Prometheus ingests against what
Grafana / alert rules / recording rules actually reference, then
suggest metric_relabel_configs snippets to drop the orphans.

Subcommands:
  metrics  Flag metric names with no observed consumer.`,
	}
	cmd.AddCommand(newUnusedMetricsCmd())
	return cmd
}

func newUnusedMetricsCmd() *cobra.Command {
	var (
		limit       int
		minSeverity string
	)
	cmd := &cobra.Command{
		Use:   "metrics",
		Short: "List ingested metrics with no Grafana / alert / recording-rule reference.",
		Long: `Compares the set of ingested metric names with the union of
metrics referenced by Grafana dashboards, alert rule expressions,
and recording rule expressions (plus the recording rule output
names themselves).

Without --grafana, dashboards are not consulted; only alert and
recording rule references count toward "used".`,
		Example: `  # Diff against Prometheus rules only
  remetric unused metrics --prometheus http://localhost:9090

  # Include Grafana dashboards
  remetric unused metrics --prometheus http://localhost:9090 --grafana http://localhost:3000

  # JSON output
  remetric unused metrics --prometheus http://localhost:9090 --output json`,
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

			prom, err := buildPromClient(cfg, "remetric/unused-metrics")
			if err != nil {
				return &exitError{code: 2, err: err}
			}
			graf, err := buildGrafanaClient(cfg, "remetric/unused-metrics")
			if err != nil {
				return &exitError{code: 2, err: err}
			}

			d := analyzers.Deps{
				Prom:   prom,
				Graf:   graf,
				Logger: loggerFrom(ctx),
				Limits: analyzers.DefaultLimits(),
			}
			res, err := unusedmetrics.New().Analyze(ctx, d)
			if err != nil {
				return &exitError{code: 1, err: err}
			}
			for _, w := range res.Warnings {
				fmt.Fprintf(cmd.ErrOrStderr(), "! warning: %s\n", w)
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

			return renderFindings(cfg, cmd.OutOrStdout(), filtered)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum findings to print (0 = none)")
	cmd.Flags().StringVar(&minSeverity, "min-severity", "medium", "Minimum severity to print: low|medium|high|critical")
	return cmd
}
