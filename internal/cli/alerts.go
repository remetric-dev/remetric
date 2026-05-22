// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package cli

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/remetric-dev/remetric/internal/analyzers"
	"github.com/remetric-dev/remetric/internal/analyzers/alerthygiene"
	"github.com/remetric-dev/remetric/internal/findings"
)

func newAlertsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alerts",
		Short: "Inspect alert-rule hygiene (noise, broken thresholds).",
		Long: `Alerts subcommands sample the ALERTS series via query_range over a
configurable lookback window and flag alerts that never fired or that
fired continuously (always-firing).

Subcommands:
  unused          List alerts with zero firing samples in the lookback window.
  always-firing   List alerts firing >=95% of the lookback window.`,
	}
	cmd.AddCommand(newAlertsByClassCmd("unused", "never_fired",
		"List alerts that never fired in the lookback window."))
	cmd.AddCommand(newAlertsByClassCmd("always-firing", "always_firing",
		"List alerts firing >=95% of the lookback window."))
	return cmd
}

//nolint:gocyclo // RunE closure is straight-line config-gathering; no branching to flatten
func newAlertsByClassCmd(use, classSuffix, short string) *cobra.Command {
	var (
		lookback    time.Duration
		step        time.Duration
		minSeverity string
		limit       int
	)
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Example: fmt.Sprintf(`  remetric alerts %s --prometheus http://localhost:9090
  remetric alerts %s --prometheus http://localhost:9090 --lookback 24h --step 5m`, use, use),
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

			prom, err := buildPromClient(cfg, "remetric/alerts-"+use)
			if err != nil {
				return &exitError{code: 2, err: err}
			}
			vmalert, err := buildVMAlertClient(cfg, "remetric/alerts-"+use)
			if err != nil {
				return &exitError{code: 2, err: err}
			}

			res, err := alerthygiene.New(alerthygiene.Config{Lookback: lookback, Step: step}).
				Analyze(ctx, analyzers.Deps{
					Prom:    prom,
					VMAlert: vmalert,
					Logger:  loggerFrom(ctx),
					Limits:  analyzers.DefaultLimits(),
				})
			if err != nil {
				return &exitError{code: 1, err: err}
			}
			for _, w := range res.Warnings {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "! warning: %s\n", w)
			}

			// First filter by class (so totals reflect this command's scope).
			classScoped := make([]findings.Finding, 0, len(res.Findings))
			for _, f := range res.Findings {
				if strings.HasSuffix(f.ID, "/"+classSuffix) {
					classScoped = append(classScoped, f)
				}
			}

			filtered := make([]findings.Finding, 0, len(classScoped))
			for _, f := range classScoped {
				if f.Severity >= minSev {
					filtered = append(filtered, f)
				}
			}
			sort.SliceStable(filtered, func(i, j int) bool {
				if filtered[i].Severity != filtered[j].Severity {
					return filtered[i].Severity > filtered[j].Severity
				}
				return filtered[i].Title < filtered[j].Title
			})
			if len(filtered) == 0 {
				return renderEmpty(cfg, cmd.OutOrStdout(), alertsCopy(use), minSev, len(classScoped), tallyBySeverity(classScoped))
			}
			if limit >= 0 && len(filtered) > limit {
				filtered = filtered[:limit]
			}
			if len(filtered) == 0 {
				// Limit truncated everything to zero — show the "no results" copy, not the severity hint.
				return renderEmpty(cfg, cmd.OutOrStdout(), alertsCopy(use), minSev, 0, nil)
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
	cmd.Flags().DurationVar(&lookback, "lookback", 168*time.Hour, "How far back to inspect the ALERTS series")
	cmd.Flags().DurationVar(&step, "step", time.Hour, "query_range step for ALERTS sampling")
	cmd.Flags().IntVar(&limit, "limit", 50, "Maximum findings to print (-1 for unlimited)")
	cmd.Flags().StringVar(&minSeverity, "min-severity", "medium", "Minimum severity to print: low|medium|high|critical")
	return cmd
}

func alertsCopy(use string) emptyCopy {
	return emptyCopy{
		NoResults: fmt.Sprintf("No %s alerts detected in the lookback window.", use),
		Subject:   use + " alerts",
	}
}
