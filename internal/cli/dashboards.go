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
	"github.com/remetric-dev/remetric/internal/analyzers/dashboardhygiene"
	"github.com/remetric-dev/remetric/internal/findings"
)

func newDashboardsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dashboards",
		Short: "Inspect dashboard-level findings (broken panels).",
		Long: `Dashboard subcommands flag Grafana dashboards whose panel
queries reference metrics that don't exist in Prometheus.

Subcommands:
  broken  Flag panels referencing missing metrics.`,
	}
	cmd.AddCommand(newDashboardsBrokenCmd())
	return cmd
}

//nolint:gocyclo // RunE closure is straight-line config-gathering; mirrors metrics.go
func newDashboardsBrokenCmd() *cobra.Command {
	var (
		limit       int
		minSeverity string
	)
	cmd := &cobra.Command{
		Use:   "broken",
		Short: "Flag dashboards whose panels reference missing metrics.",
		Long: `Walks every Grafana dashboard, parses each Prometheus target
query, and reports (dashboard, missing-metric) pairs as one
finding each. A metric is "missing" if it is not present in
Prometheus head series (LabelValues) and not declared as the
output of any recording rule.

Requires both --prometheus and --grafana.`,
		Example: `  # All broken-panel findings
  remetric dashboards broken --prometheus http://localhost:9090 --grafana http://localhost:3000

  # JSON output for CI
  remetric dashboards broken --prometheus ... --grafana ... --output json

  # Suppress a known-broken dashboard
  remetric dashboards broken --prometheus ... --grafana ... --ignore-dashboard "Legacy Disk Stats"`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cfg := configFrom(ctx)
			if cfg == nil || cfg.Prometheus.URL == "" {
				return &flagError{err: errors.New("--prometheus is required")}
			}
			if cfg.Grafana.URL == "" {
				return &flagError{err: errors.New("--grafana is required for dashboards broken")}
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

			prom, err := buildPromClient(cfg, "remetric/dashboards-broken")
			if err != nil {
				return &exitError{code: 2, err: err}
			}
			graf, err := buildGrafanaClient(cfg, "remetric/dashboards-broken")
			if err != nil {
				return &exitError{code: 2, err: err}
			}
			vmalert, err := buildVMAlertClient(cfg, "remetric/dashboards-broken")
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
			res, err := dashboardhygiene.New().Analyze(ctx, d)
			if err != nil {
				return &exitError{code: 1, err: err}
			}
			for _, w := range res.Warnings {
				_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "! warning: %s\n", w)
			}
			all, ignored := cfg.IgnoreFilter().Apply(res.Findings)

			filtered := filterAtLeast(all, minSev)
			if len(filtered) == 0 {
				return renderEmpty(cfg, cmd.OutOrStdout(), brokenPanelCopy, minSev, len(all), tallyBySeverity(all), ignored)
			}
			sort.SliceStable(filtered, func(i, j int) bool {
				if filtered[i].Severity != filtered[j].Severity {
					return filtered[i].Severity > filtered[j].Severity
				}
				if filtered[i].Dashboard != filtered[j].Dashboard {
					return filtered[i].Dashboard < filtered[j].Dashboard
				}
				return filtered[i].Metric < filtered[j].Metric
			})
			if limit >= 0 && len(filtered) > limit {
				filtered = filtered[:limit]
			}
			if len(filtered) == 0 {
				return renderEmpty(cfg, cmd.OutOrStdout(), brokenPanelCopy, minSev, 0, nil, ignored)
			}
			if err := renderFindings(cfg, cmd.OutOrStdout(), filtered, ignored); err != nil {
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

var brokenPanelCopy = emptyCopy{
	NoResults: "No broken panels found - every dashboard panel query resolves to a metric that exists in Prometheus head series or as a recording-rule output.",
	Subject:   "broken panels",
}
