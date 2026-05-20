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
	"github.com/remetric-dev/remetric/internal/output/terminal"
)

func newCardinalityCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cardinality",
		Short: "Cardinality analysis (metrics & labels).",
	}
	cmd.AddCommand(newCardinalityTopCmd())
	return cmd
}

func newCardinalityTopCmd() *cobra.Command {
	var (
		limit       int
		minSeverity string
	)
	cmd := &cobra.Command{
		Use:     "top",
		Short:   "List the worst cardinality offenders.",
		Example: "  remetric cardinality top --prometheus http://localhost:9090 --limit 20",
		RunE: func(cmd *cobra.Command, _ []string) error {
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
			a := cardinality.New()
			all, err := a.Analyze(ctx, d)
			if err != nil {
				return &exitError{code: 1, err: err}
			}

			filtered := make([]findings.Finding, 0, len(all))
			for _, f := range all {
				if f.Severity >= minSev {
					filtered = append(filtered, f)
				}
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

			r := terminal.New(cmd.OutOrStdout(), terminal.WithColor(!cfg.NoColor))
			return r.RenderFindings(filtered)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum findings to print (0 = none)")
	cmd.Flags().StringVar(&minSeverity, "min-severity", "medium", "Minimum severity to print: low|medium|high|critical")
	return cmd
}
