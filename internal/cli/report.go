// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"sort"
	"time"

	"github.com/spf13/cobra"

	"github.com/remetric-dev/remetric/internal/analyzers"
	"github.com/remetric-dev/remetric/internal/analyzers/alerthygiene"
	"github.com/remetric-dev/remetric/internal/analyzers/cardinality"
	"github.com/remetric-dev/remetric/internal/analyzers/labelpattern"
	"github.com/remetric-dev/remetric/internal/analyzers/unusedmetrics"
	"github.com/remetric-dev/remetric/internal/findings"
	"github.com/remetric-dev/remetric/internal/output/html"
	outjson "github.com/remetric-dev/remetric/internal/output/json"
	"github.com/remetric-dev/remetric/internal/output/markdown"
	"github.com/remetric-dev/remetric/internal/output/terminal"
	"github.com/remetric-dev/remetric/internal/progress"
)

//nolint:gocyclo // RunE closure is straight-line config-gathering; no branching to flatten
func newReportCmd() *cobra.Command {
	var (
		format      string
		outPath     string
		minSeverity string
		lookback    time.Duration
		step        time.Duration
	)
	cmd := &cobra.Command{
		Use:   "report",
		Short: "Run all analyzers and emit a report in the chosen format.",
		Long: `Runs cardinality, label patterns, unused metrics, and alert hygiene,
then renders the unified report to stdout or a file in terminal, JSON,
HTML, or Markdown format.

The global --output flag is ignored by this subcommand; use --format
instead. Use --out FILE to write to a file (or '-' for stdout).`,
		Example: `  remetric report --prometheus http://localhost:9090 --format html --out report.html
  remetric report --prometheus http://localhost:9090 --format markdown > report.md`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cfg := configFrom(ctx)
			if cfg == nil || cfg.Prometheus.URL == "" {
				return &flagError{err: errors.New("--prometheus is required")}
			}
			if err := validateOutput(cfg.Output); err != nil {
				return &flagError{err: err}
			}
			switch format {
			case "", "terminal", "json", "html", "markdown":
			default:
				return &flagError{err: fmt.Errorf("invalid --format %q (want terminal|json|html|markdown)", format)}
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

			prom, err := buildPromClient(cfg, "remetric/report")
			if err != nil {
				return &exitError{code: 2, err: err}
			}
			graf, err := buildGrafanaClient(cfg, "remetric/report")
			if err != nil {
				return &exitError{code: 2, err: err}
			}
			vmalert, err := buildVMAlertClient(cfg, "remetric/report")
			if err != nil {
				return &exitError{code: 2, err: err}
			}

			deps := analyzers.Deps{
				Prom:    prom,
				Graf:    graf,
				VMAlert: vmalert,
				Logger:  loggerFrom(ctx),
				Limits:  analyzers.DefaultLimits(),
			}
			runners := []analyzers.Analyzer{
				cardinality.New(),
				labelpattern.New(),
				unusedmetrics.New(),
				alerthygiene.New(alerthygiene.Config{Lookback: lookback, Step: step}),
			}
			var (
				all      []findings.Finding
				warnings []string
			)
			prog := progress.New(cmd.ErrOrStderr(), cfg.NoProgress)
			for _, a := range runners {
				prog.Start(a.Name())
				t0 := time.Now()
				res, err := a.Analyze(ctx, deps)
				if err != nil {
					return &exitError{code: 1, err: fmt.Errorf("%s: %w", a.Name(), err)}
				}
				prog.Done(a.Name(), time.Since(t0), len(res.Warnings))
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

			rep := buildReport(cmd, cfg, filtered, prom)
			rep.Warnings = warnings
			rep.ScannedAt = time.Now().UTC()

			out, closer, err := openOutput(cmd, outPath)
			if err != nil {
				return &exitError{code: 2, err: err}
			}
			renderErr := renderFormat(out, format, rep, cfg.NoColor)
			if closeErr := closer(); closeErr != nil && renderErr == nil {
				return closeErr
			}
			return renderErr
		},
	}
	cmd.Flags().StringVar(&format, "format", "terminal", "Output format: terminal|json|html|markdown")
	cmd.Flags().StringVar(&outPath, "out", "-", "Output file path, or '-' for stdout")
	cmd.Flags().StringVar(&minSeverity, "min-severity", "medium", "Minimum severity to include: low|medium|high|critical")
	cmd.Flags().DurationVar(&lookback, "lookback", 168*time.Hour, "Alert-hygiene lookback window")
	cmd.Flags().DurationVar(&step, "step", time.Hour, "Alert-hygiene query_range step")
	return cmd
}

func openOutput(cmd *cobra.Command, path string) (io.Writer, func() error, error) {
	if path == "" || path == "-" {
		return cmd.OutOrStdout(), func() error { return nil }, nil
	}
	f, err := os.Create(path) //nolint:gosec // G304: path is a user-supplied --out flag; CLI files are by design.
	if err != nil {
		return nil, nil, fmt.Errorf("open %s: %w", path, err)
	}
	return f, f.Close, nil
}

func renderFormat(w io.Writer, format string, rep *findings.Report, noColor bool) error {
	switch format {
	case "", "terminal":
		if err := writeTerminalWarnings(w, rep.Warnings, !noColor); err != nil {
			return err
		}
		return terminal.New(w, terminal.WithColor(!noColor)).RenderFindings(rep.Findings)
	case "json":
		return outjson.New(w).RenderReport(rep)
	case "html":
		return html.New(w).RenderReport(rep)
	case "markdown":
		return markdown.New(w).RenderReport(rep)
	}
	return fmt.Errorf("unsupported --format %q", format)
}
