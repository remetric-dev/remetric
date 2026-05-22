// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package cli

import (
	"fmt"
	"io"

	"github.com/remetric-dev/remetric/internal/config"
	"github.com/remetric-dev/remetric/internal/findings"
	outjson "github.com/remetric-dev/remetric/internal/output/json"
	"github.com/remetric-dev/remetric/internal/output/terminal"
)

// renderFindings dispatches finding rendering based on cfg.Output.
// Supported values: "terminal", "json". The ignored parameter is the
// number of findings dropped by --ignore-* (0 if filter unused).
//
// renderFindings populates Finding.DocURL from Finding.Class for every
// finding that has a Class but no caller-provided DocURL. The fill happens
// here (after callers have applied the ignore filter, before any
// format-specific renderer touches the slice) so no analyzer has to
// hard-code the docs domain.
func renderFindings(cfg *config.Config, w io.Writer, fs []findings.Finding, ignored int) error {
	findings.FillDocURLs(fs)
	switch cfg.Output {
	case "", "terminal":
		r := terminal.New(w, terminal.WithColor(!cfg.NoColor))
		if err := r.RenderFindings(fs); err != nil {
			return err
		}
		return writeTerminalIgnored(w, ignored)
	case "json":
		return outjson.New(w).RenderFindingsWithIgnored(fs, ignored)
	default:
		return fmt.Errorf("unsupported --output %q (want terminal|json)", cfg.Output)
	}
}

// validateOutput returns an error if s is not a supported output format.
// Accepted: "", "terminal", "json".
func validateOutput(s string) error {
	switch s {
	case "", "terminal", "json":
		return nil
	default:
		return fmt.Errorf("invalid --output: %q (want terminal|json)", s)
	}
}

// renderReport dispatches Report rendering based on cfg.Output.
// Terminal output prints any Warnings as a yellow banner above the
// findings table. JSON emits the full §5.5 envelope.
func renderReport(cfg *config.Config, w io.Writer, rep *findings.Report) error {
	switch cfg.Output {
	case "", "terminal":
		if err := writeTerminalWarnings(w, rep.Warnings, !cfg.NoColor); err != nil {
			return err
		}
		r := terminal.New(w, terminal.WithColor(!cfg.NoColor))
		if err := r.RenderFindings(rep.Findings); err != nil {
			return err
		}
		return writeTerminalIgnored(w, rep.IgnoredCount)
	case "json":
		return outjson.New(w).RenderReport(rep)
	default:
		return fmt.Errorf("unsupported --output %q (want terminal|json)", cfg.Output)
	}
}

// writeTerminalIgnored prints a footer line summarizing the number of
// findings dropped by --ignore-* patterns. Zero is a no-op.
func writeTerminalIgnored(w io.Writer, ignored int) error {
	if ignored == 0 {
		return nil
	}
	word := "findings"
	if ignored == 1 {
		word = "finding"
	}
	_, err := fmt.Fprintf(w, "\nignored: %d %s (dropped by --ignore-*)\n", ignored, word)
	return err
}

// writeTerminalWarnings prints each warning as a banner line above the
// findings table. Empty input is a no-op. When color is true, the
// "! warning:" prefix is yellow.
func writeTerminalWarnings(w io.Writer, warnings []string, color bool) error {
	if len(warnings) == 0 {
		return nil
	}
	for _, msg := range warnings {
		var line string
		if color {
			line = fmt.Sprintf("\x1b[33m! warning:\x1b[0m %s\n", msg)
		} else {
			line = fmt.Sprintf("! warning: %s\n", msg)
		}
		if _, err := io.WriteString(w, line); err != nil {
			return err
		}
	}
	// Blank line separator before findings table.
	if _, err := io.WriteString(w, "\n"); err != nil {
		return err
	}
	return nil
}
