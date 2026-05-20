package terminal

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/jedib0t/go-pretty/v6/text"

	"github.com/remetric-dev/remetric/internal/findings"
)

// RenderFindings writes a sorted table plus fix blocks for critical/high.
func (r *Renderer) RenderFindings(fs []findings.Finding) error {
	if len(fs) == 0 {
		_, err := fmt.Fprintln(r.w, "No cardinality findings above min-severity. Run `remetric doctor` to verify the connection.")
		return err
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
		writeFixBlock(r, f)
	}
	return nil
}

func writeFixBlock(r *Renderer, f findings.Finding) {
	tag := "[" + f.Severity.String() + "]"
	tag = r.st.severity(f.Severity).Render(tag)
	fmt.Fprintf(r.w, "\n%s %s  ·  %s has %d unique values\n", tag, f.Metric, f.Evidence.Label, f.Evidence.UniqueValues)
	if len(f.Evidence.SampleValues) > 0 {
		fmt.Fprintf(r.w, "Sample: %s\n", strings.Join(f.Evidence.SampleValues, ", "))
	}
	fmt.Fprintf(r.w, "Estimated reduction (upper bound): %s series\n", fmtInt(f.Impact.SeriesReduction))
	if f.Fix.Config != "" {
		fmt.Fprintln(r.w, "\nSuggested fix (Prometheus scrape config):")
		for _, line := range strings.Split(strings.TrimRight(f.Fix.Config, "\n"), "\n") {
			fmt.Fprintf(r.w, "    %s\n", line)
		}
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
