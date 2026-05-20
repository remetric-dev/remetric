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
	"github.com/remetric-dev/remetric/internal/analyzers/labelpattern"
	"github.com/remetric-dev/remetric/internal/findings"
)

func newCardinalitySuspiciousCmd() *cobra.Command {
	var (
		limit       int
		minSeverity string
	)
	cmd := &cobra.Command{
		Use:     "suspicious",
		Short:   "Flag labels whose names match well-known unbounded-identifier patterns.",
		Example: "  remetric cardinality suspicious --prometheus http://localhost:9090",
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

			client, err := buildPromClient(cfg, "remetric/cardinality-suspicious")
			if err != nil {
				return &exitError{code: 2, err: err}
			}

			d := analyzers.Deps{
				Prom:   client,
				Logger: loggerFrom(ctx),
				Limits: analyzers.DefaultLimits(),
			}
			a := labelpattern.New()
			all, err := a.Analyze(ctx, d)
			if err != nil {
				return &exitError{code: 1, err: err}
			}

			filtered := filterAtLeast(all, minSev)
			if len(filtered) == 0 {
				return renderEmpty(cfg, cmd.OutOrStdout(), labelPatternCopy, minSev, len(all), tallyBySeverity(all))
			}

			sort.SliceStable(filtered, func(i, j int) bool {
				if filtered[i].Severity != filtered[j].Severity {
					return filtered[i].Severity > filtered[j].Severity
				}
				return filtered[i].Evidence.UniqueValues > filtered[j].Evidence.UniqueValues
			})
			if limit >= 0 && len(filtered) > limit {
				filtered = filtered[:limit]
			}
			if len(filtered) == 0 {
				// `--limit 0` (or smaller) truncated every finding away.
				// Treat as "no results" so users see the new empty-state copy.
				return renderEmpty(cfg, cmd.OutOrStdout(), labelPatternCopy, minSev, 0, nil)
			}

			return renderFindings(cfg, cmd.OutOrStdout(), filtered)
		},
	}
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum findings to print (0 = none)")
	cmd.Flags().StringVar(&minSeverity, "min-severity", "medium", "Minimum severity to print: low|medium|high|critical")
	return cmd
}
