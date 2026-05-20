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
// Supported values: "terminal", "json".
func renderFindings(cfg *config.Config, w io.Writer, fs []findings.Finding) error {
	switch cfg.Output {
	case "", "terminal":
		r := terminal.New(w, terminal.WithColor(!cfg.NoColor))
		return r.RenderFindings(fs)
	case "json":
		return outjson.New(w).RenderFindings(fs)
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
