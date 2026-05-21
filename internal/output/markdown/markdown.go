// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

// Package markdown renders a findings.Report as GitHub-flavored Markdown.
package markdown

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/remetric-dev/remetric/internal/findings"
)

// Renderer writes a Report as Markdown to an io.Writer.
type Renderer struct{ w io.Writer }

// New constructs a Renderer.
func New(w io.Writer) *Renderer { return &Renderer{w: w} }

// RenderReport writes the full report to the underlying writer.
func (r *Renderer) RenderReport(rep *findings.Report) error {
	if rep == nil {
		return fmt.Errorf("output/markdown: nil report")
	}
	var b strings.Builder
	writeHeader(&b, rep)
	writeWarnings(&b, rep.Warnings)
	writeSummary(&b, rep.Summary)
	writeFindings(&b, rep.Findings)
	_, err := io.WriteString(r.w, b.String())
	return err
}

func writeHeader(b *strings.Builder, rep *findings.Report) {
	b.WriteString("# remetric report\n\n")
	if rep.Target != nil {
		fmt.Fprintf(b, "- Target: %s\n", rep.Target.PrometheusURL)
		if rep.Target.PrometheusVersion != "" {
			fmt.Fprintf(b, "- Prometheus version: %s\n", rep.Target.PrometheusVersion)
		}
	}
	fmt.Fprintf(b, "- Scanned: %s\n", rep.ScannedAt.Format("2006-01-02T15:04:05Z07:00"))
	if rep.Version != "" {
		fmt.Fprintf(b, "- Version: %s\n", rep.Version)
	}
	b.WriteString("\n")
}

func writeWarnings(b *strings.Builder, warnings []string) {
	if len(warnings) == 0 {
		return
	}
	b.WriteString("## Warnings\n\n")
	for _, w := range warnings {
		fmt.Fprintf(b, "- %s\n", w)
	}
	b.WriteString("\n")
}

func writeSummary(b *strings.Builder, s findings.Summary) {
	b.WriteString("## Summary\n\n")
	b.WriteString("| Severity | Count |\n|----------|-------|\n")
	for _, sev := range []string{"critical", "high", "medium", "low"} {
		label := strings.ToUpper(sev[:1]) + sev[1:]
		fmt.Fprintf(b, "| %s | %d |\n", label, s.BySeverity[sev])
	}
	if s.PotentialSeriesReduction > 0 {
		fmt.Fprintf(b, "\nPotential series reduction: **%d**\n", s.PotentialSeriesReduction)
	}
}

func writeFindings(b *strings.Builder, fs []findings.Finding) {
	b.WriteString("\n## Findings\n\n")
	byCategory := map[findings.Category][]findings.Finding{}
	for _, f := range fs {
		byCategory[f.Category] = append(byCategory[f.Category], f)
	}
	cats := make([]string, 0, len(byCategory))
	for c := range byCategory {
		cats = append(cats, string(c))
	}
	sort.Strings(cats)
	for _, c := range cats {
		fmt.Fprintf(b, "### %s\n\n", c)
		for _, f := range byCategory[findings.Category(c)] {
			writeFinding(b, f)
		}
	}
}

func writeFinding(b *strings.Builder, f findings.Finding) {
	fmt.Fprintf(b, "#### [%s] %s\n\n", f.Severity, f.Title)
	if f.Evidence.Description != "" {
		fmt.Fprintf(b, "%s\n\n", f.Evidence.Description)
	}
	if f.Fix.Config == "" {
		return
	}
	b.WriteString("```yaml\n")
	b.WriteString(f.Fix.Config)
	if !strings.HasSuffix(f.Fix.Config, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("```\n\n")
}
