// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package terminal

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"

	"github.com/remetric-dev/remetric/internal/findings"
)

// RenderFindings writes a sorted table plus fix blocks for critical/high.
func (r *Renderer) RenderFindings(fs []findings.Finding) error {
	if len(fs) == 0 {
		// Empty input is a defensive no-op; the CLI owns the user-facing
		// empty-state message (analyzer-aware) - see internal/cli/empty.go.
		return nil
	}

	sorted := make([]findings.Finding, len(fs))
	copy(sorted, fs)
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].Severity != sorted[j].Severity {
			return sorted[i].Severity > sorted[j].Severity
		}
		return sorted[i].Evidence.SeriesCount > sorted[j].Evidence.SeriesCount
	})

	tbl := table.NewWriter()
	tbl.SetOutputMirror(r.w)
	tbl.SetStyle(table.StyleLight)
	tbl.Style().Format.Header = text.FormatUpper
	tbl.AppendHeader(table.Row{"Severity", "Metric", "Label", "Series", "Unique", "Est. Reduction"})
	for _, f := range sorted {
		sev := r.st.severity(f.Severity).Render(f.Severity.String())
		tbl.AppendRow(table.Row{
			sev,
			truncate(f.Metric, 28),
			truncate(f.Evidence.Label, 24),
			fmtInt(f.Evidence.SeriesCount),
			fmtInt(int64(f.Evidence.UniqueValues)),
			"~" + fmtInt(f.Impact.SeriesReduction),
		})
	}
	tbl.Render()

	for _, f := range sorted {
		if f.Severity != findings.SeverityCritical && f.Severity != findings.SeverityHigh {
			continue
		}
		if err := writeFixBlock(r, f); err != nil {
			return err
		}
	}
	return nil
}

func writeFixBlock(r *Renderer, f findings.Finding) error {
	tag := r.st.severity(f.Severity).Render("[" + f.Severity.String() + "]")
	w := &errWriter{w: r.w}
	if f.Metric != "" {
		w.printf("\n%s %s  ·  %s has %s unique values\n", tag, f.Metric, f.Evidence.Label, fmtInt(int64(f.Evidence.UniqueValues)))
	} else {
		w.printf("\n%s label '%s' has %s unique values\n", tag, f.Evidence.Label, fmtInt(int64(f.Evidence.UniqueValues)))
	}
	if len(f.Evidence.SampleValues) > 0 {
		w.printf("Sample: %s\n", strings.Join(f.Evidence.SampleValues, ", "))
	}
	w.printf("Estimated reduction (upper bound): %s series\n", fmtInt(f.Impact.SeriesReduction))
	if f.Fix.Config != "" {
		w.printf("\nSuggested fix (Prometheus scrape config):\n")
		for _, line := range strings.Split(strings.TrimRight(f.Fix.Config, "\n"), "\n") {
			w.printf("    %s\n", line)
		}
	}
	if f.DocURL != "" {
		w.printf("Reference: %s\n", f.DocURL)
	}
	return w.err
}

// errWriter wraps an io.Writer and remembers the first error. Subsequent
// printf calls become no-ops, so callers only check err once at the end.
type errWriter struct {
	w   io.Writer
	err error
}

func (e *errWriter) printf(format string, args ...any) {
	if e.err != nil {
		return
	}
	_, err := fmt.Fprintf(e.w, format, args...)
	if err != nil {
		e.err = err
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n <= 3 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

// fmtInt formats integers with thousand separators ("487,234").
func fmtInt(n int64) string {
	if n < 0 {
		return "-" + fmtInt(-n)
	}
	s := fmt.Sprintf("%d", n)
	var out strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			out.WriteByte(',')
		}
		out.WriteRune(c)
	}
	return out.String()
}
