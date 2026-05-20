// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"
	"github.com/spf13/cobra"

	"github.com/remetric-dev/remetric/internal/config"
	outjson "github.com/remetric-dev/remetric/internal/output/json"
)

func newCardinalityLabelsCmd() *cobra.Command {
	var (
		metric     string
		sampleSize int
	)
	cmd := &cobra.Command{
		Use:     "labels",
		Short:   "Show labels for a metric sorted by unique value count.",
		Example: "  remetric cardinality labels --metric http_requests_total",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			cfg := configFrom(ctx)
			if cfg == nil || cfg.Prometheus.URL == "" {
				return &flagError{err: errors.New("--prometheus is required")}
			}
			if metric == "" {
				return &flagError{err: errors.New("--metric is required")}
			}
			if err := validateOutput(cfg.Output); err != nil {
				return &flagError{err: err}
			}
			if cfg.Timeout > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithTimeout(ctx, cfg.Timeout)
				defer cancel()
			}

			client, err := buildPromClient(cfg, "remetric/cardinality-labels")
			if err != nil {
				return &exitError{code: 2, err: err}
			}

			labels, err := client.LabelNamesForMetric(ctx, metric)
			if err != nil {
				return &exitError{code: 1, err: err}
			}

			matcher := fmt.Sprintf(`{__name__="%s"}`, metric)
			inv := outjson.LabelInventory{Metric: metric}
			for _, lbl := range labels {
				vals, err := client.LabelValues(ctx, lbl, matcher)
				if err != nil {
					return &exitError{code: 1, err: fmt.Errorf("values for %q: %w", lbl, err)}
				}
				inv.Labels = append(inv.Labels, outjson.LabelStat{
					Name:         lbl,
					UniqueValues: len(vals),
					SampleValues: firstN(vals, sampleSize),
				})
			}
			sort.SliceStable(inv.Labels, func(i, j int) bool {
				return inv.Labels[i].UniqueValues > inv.Labels[j].UniqueValues
			})

			return renderInventory(cfg, cmd.OutOrStdout(), inv)
		},
	}
	cmd.Flags().StringVar(&metric, "metric", "", "Metric name to inspect (required)")
	cmd.Flags().IntVar(&sampleSize, "sample-size", 5, "Number of sample values to keep per label")
	return cmd
}

func renderInventory(cfg *config.Config, w io.Writer, inv outjson.LabelInventory) error {
	switch cfg.Output {
	case "", "terminal":
		if len(inv.Labels) == 0 {
			_, err := fmt.Fprintf(w, "No labels found for %s.\n", inv.Metric)
			return err
		}
		tbl := table.NewWriter()
		tbl.SetOutputMirror(w)
		tbl.SetStyle(table.StyleLight)
		tbl.Style().Format.Header = text.FormatUpper
		tbl.AppendHeader(table.Row{"Name", "Unique", "Sample"})
		for _, l := range inv.Labels {
			tbl.AppendRow(table.Row{l.Name, l.UniqueValues, joinSamples(l.SampleValues)})
		}
		tbl.Render()
		return nil
	case "json":
		return outjson.New(w).RenderLabelInventory(inv)
	default:
		return fmt.Errorf("unsupported --output %q (want terminal|json)", cfg.Output)
	}
}

func firstN(xs []string, n int) []string {
	if n <= 0 || len(xs) == 0 {
		return nil
	}
	if n > len(xs) {
		n = len(xs)
	}
	out := make([]string, n)
	copy(out, xs[:n])
	return out
}

func joinSamples(xs []string) string {
	if len(xs) == 0 {
		return "—"
	}
	out := xs[0]
	for _, s := range xs[1:] {
		out += ", " + s
	}
	return out
}
