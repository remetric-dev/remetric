// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

// Package html renders a findings.Report as a self-contained HTML document.
package html

import (
	"fmt"
	"html/template"
	"io"
	"math"
	"sort"
	"strings"

	"github.com/remetric-dev/remetric/internal/findings"
)

// Renderer writes a Report as HTML to an io.Writer.
type Renderer struct{ w io.Writer }

// New constructs a Renderer.
func New(w io.Writer) *Renderer { return &Renderer{w: w} }

type viewData struct {
	Version          string
	Target           *findings.Target
	Overview         *findings.Overview
	ScannedAtDisplay string
	Warnings         []string
	Counts           map[string]int
	IgnoredCount     int
	Severities       []string
	Categories       []string
	Findings         []findings.Finding
	DonutSVG         template.HTML
	CSS              template.CSS
	JS               template.JS
}

// RenderReport writes the report to the underlying writer.
func (r *Renderer) RenderReport(rep *findings.Report) error {
	if rep == nil {
		return fmt.Errorf("output/html: nil report")
	}
	target := rep.Target
	if target == nil {
		target = &findings.Target{}
	}
	cats := map[string]struct{}{}
	for _, f := range rep.Findings {
		cats[string(f.Category)] = struct{}{}
	}
	catList := make([]string, 0, len(cats))
	for c := range cats {
		catList = append(catList, c)
	}
	sort.Strings(catList)

	view := viewData{
		Version:          rep.Version,
		Target:           target,
		Overview:         rep.Overview,
		ScannedAtDisplay: rep.ScannedAt.Format("2006-01-02 15:04 MST"),
		Warnings:         rep.Warnings,
		Counts:           rep.Summary.BySeverity,
		IgnoredCount:     rep.IgnoredCount,
		Severities:       []string{"CRITICAL", "HIGH", "MEDIUM", "LOW"},
		Categories:       catList,
		Findings:         rep.Findings,
		DonutSVG:         template.HTML(donutSVG(rep.Summary.BySeverity)), //nolint:gosec // static SVG built from a fixed palette and numeric counts; no user input
		CSS:              template.CSS(styleSrc),                          //nolint:gosec // embedded static stylesheet under our control
		JS:               template.JS(scriptSrc),                          //nolint:gosec // embedded static script under our control
	}
	return rendererTemplate.Execute(r.w, view)
}

// donutSVG renders a 100×100 inline SVG donut showing severity distribution.
// Slices are drawn in the order critical, high, medium, low.
func donutSVG(counts map[string]int) string {
	order := []string{"critical", "high", "medium", "low"}
	colors := map[string]string{"critical": "#d9534f", "high": "#f0ad4e", "medium": "#5bc0de", "low": "#5cb85c"}
	total := 0
	for _, k := range order {
		total += counts[k]
	}
	var b strings.Builder
	b.WriteString(`<svg viewBox="0 0 32 32" role="img" aria-label="severity distribution">`)
	if total == 0 {
		b.WriteString(`<circle cx="16" cy="16" r="12" fill="none" stroke="#ddd" stroke-width="6"/>`)
		b.WriteString(`</svg>`)
		return b.String()
	}
	offset := 0.0
	circumference := 2 * math.Pi * 12
	for _, k := range order {
		n := counts[k]
		if n == 0 {
			continue
		}
		frac := float64(n) / float64(total)
		dash := frac * circumference
		fmt.Fprintf(&b,
			`<circle cx="16" cy="16" r="12" fill="none" stroke="%s" stroke-width="6" stroke-dasharray="%.3f %.3f" stroke-dashoffset="%.3f" transform="rotate(-90 16 16)"/>`,
			colors[k], dash, circumference-dash, -offset)
		offset += dash
	}
	b.WriteString(`</svg>`)
	return b.String()
}
