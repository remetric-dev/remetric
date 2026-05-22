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
	"github.com/remetric-dev/remetric/internal/analyzers/cardinality"
	"github.com/remetric-dev/remetric/internal/findings"
)

func newCardinalityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cardinality",
		Short: "Cardinality analysis (metrics & labels).",
		Long: `Cardinality subcommands inspect the Prometheus TSDB for high-cardinality
metrics and labels and propose metric_relabel_configs fixes.

Subcommands:
  top         List the worst high-cardinality metric/label pairs.
  labels      Show all labels of a single metric, sorted by uniqueness.
  suspicious  Flag labels whose names match well-known unbounded-identifier
              patterns (uuid, *_id, path, trace_id, …).

All subcommands accept --output {terminal,json} and --min-severity.`,
	}
	cmd.AddCommand(newCardinalityTopCmd())
	cmd.AddCommand(newCardinalityLabelsCmd())
	cmd.AddCommand(newCardinalitySuspiciousCmd())
	return cmd
}

//nolint:gocyclo // RunE closure is straight-line config-gathering; no branching to flatten
func newCardinalityTopCmd() *cobra.Command {
	var (
		limit       int
		minSeverity string
	)
	cmd := &cobra.Command{
		Use:   "top",
		Short: "List the worst cardinality offenders.",
		Long: `Ranks metrics by series count and attributes each one to the label that
drives the explosion. For each high-severity finding the output includes
a copy-pasteable metric_relabel_configs labeldrop snippet.

Severity thresholds (configurable via Phase 3):
  Critical: metric has >300,000 series OR contributes >15% of total
  High:     >100,000 series OR >5% of total
  Medium:   >25,000 series
  Low:      anything else flagged`,
		Example: `  # Default scan (medium+ severities)
  remetric cardinality top --prometheus http://localhost:9090

  # Include low-severity findings
  remetric cardinality top --prometheus http://localhost:9090 --min-severity low

  # JSON output for CI / scripting
  remetric cardinality top --prometheus http://localhost:9090 --output json

  # Limit results
  remetric cardinality top --prometheus http://localhost:9090 --limit 5`,
		RunE: func(cmd *cobra.Command, _ []string) error {
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
			minSev, err := findings.ParseSeverity(minSeverity)
			if err != nil {
				return &flagError{err: fmt.Errorf("invalid --min-severity: %w", err)}
			}

			client, err := buildPromClient(cfg, "remetric/cardinality")
			if err != nil {
				return &exitError{code: 2, err: err}
			}

			d := analyzers.Deps{
				Prom:   client,
				Logger: loggerFrom(ctx),
				Limits: analyzers.DefaultLimits(),
			}
			res, err := cardinality.New().Analyze(ctx, d)
			if err != nil {
				return &exitError{code: 1, err: err}
			}
			for _, w := range res.Warnings {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "! warning: %s\n", w)
			}
			all := res.Findings

			filtered := filterAtLeast(all, minSev)
			if len(filtered) == 0 {
				return renderEmpty(cfg, cmd.OutOrStdout(), cardinalityCopy, minSev, len(all), tallyBySeverity(all))
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
				// `--limit 0` (or smaller) truncated every finding away.
				// Treat as "no results" so users see the new empty-state copy.
				return renderEmpty(cfg, cmd.OutOrStdout(), cardinalityCopy, minSev, 0, nil)
			}

			if err := renderFindings(cfg, cmd.OutOrStdout(), filtered); err != nil {
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
