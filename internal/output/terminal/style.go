// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

// Package terminal renders findings and doctor reports for human-readable terminal output.
package terminal

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/remetric-dev/remetric/internal/findings"
)

type styles struct {
	critical lipgloss.Style
	high     lipgloss.Style
	medium   lipgloss.Style
	low      lipgloss.Style
	ok       lipgloss.Style
	warn     lipgloss.Style
	header   lipgloss.Style
	dim      lipgloss.Style
}

func newStyles(noColor bool) styles {
	if noColor {
		plain := lipgloss.NewStyle()
		return styles{critical: plain, high: plain, medium: plain, low: plain, ok: plain, warn: plain, header: plain, dim: plain}
	}
	return styles{
		critical: lipgloss.NewStyle().Foreground(lipgloss.Color("#e02020")).Bold(true),
		high:     lipgloss.NewStyle().Foreground(lipgloss.Color("#ff8c00")).Bold(true),
		medium:   lipgloss.NewStyle().Foreground(lipgloss.Color("#d4a017")),
		low:      lipgloss.NewStyle().Foreground(lipgloss.Color("#5fafaf")),
		ok:       lipgloss.NewStyle().Foreground(lipgloss.Color("#2cb673")),
		warn:     lipgloss.NewStyle().Foreground(lipgloss.Color("#ff8c00")),
		header:   lipgloss.NewStyle().Bold(true),
		dim:      lipgloss.NewStyle().Foreground(lipgloss.Color("#7a7a7a")),
	}
}

func (s styles) severity(sev findings.Severity) lipgloss.Style {
	switch sev {
	case findings.SeverityCritical:
		return s.critical
	case findings.SeverityHigh:
		return s.high
	case findings.SeverityMedium:
		return s.medium
	default:
		return s.low
	}
}
