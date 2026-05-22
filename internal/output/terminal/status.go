// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package terminal

import (
	"time"

	"github.com/remetric-dev/remetric/internal/findings"
)

// RenderDoctor writes a human-friendly status block.
func (r *Renderer) RenderDoctor(rep findings.DoctorReport) error {
	w := &errWriter{w: r.w}
	header := r.st.header.Render("remetric doctor")
	elapsed := time.Duration(rep.ElapsedMs) * time.Millisecond
	w.printf("%s - %s (in %s)\n\n", header, rep.PrometheusURL, elapsed)

	mark := func(ok bool) string {
		if ok {
			return r.st.ok.Render("[OK]")
		}
		return r.st.warn.Render("[FAIL]")
	}

	w.printf("%s reachable\n", mark(rep.Reachable))
	if rep.Version != "" {
		label := "version " + rep.Version
		if !rep.VersionOK {
			label += " (below minimum 2.30)"
		}
		w.printf("%s %s\n", mark(rep.VersionOK), label)
	}
	if rep.Backend != "" {
		w.printf("     backend:      %s\n", rep.Backend)
	}
	w.printf("%s tsdb stats accessible (auth: %s)\n", mark(rep.TSDBStatsOK), rep.AuthMethod)
	if rep.NumSeries > 0 {
		w.printf("     active series: %s\n", fmtInt(rep.NumSeries))
	}
	if rep.NumMetrics > 0 {
		w.printf("     metric names: %s\n", fmtInt(rep.NumMetrics))
	}
	switch {
	case rep.StorageRetention != "":
		w.printf("     retention:    %s\n", rep.StorageRetention)
	case !rep.RuntimeInfoSupported:
		w.printf("     retention:    n/a (not supported by backend)\n")
	}

	if len(rep.Errors) > 0 {
		w.printf("\nIssues:\n")
		for _, e := range rep.Errors {
			w.printf("  %s %s: %s\n", r.st.warn.Render("·"), e.Check, e.Error)
		}
	}

	return w.err
}
