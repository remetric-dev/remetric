package terminal

import (
	"time"
)

// DoctorReport holds the result of the `remetric doctor` command for rendering.
type DoctorReport struct {
	PrometheusURL string
	Reachable     bool
	Version       string
	VersionOK     bool
	AuthMethod    string
	TSDBStatsOK   bool
	NumSeries     int64
	Elapsed       time.Duration
	Errors        []DoctorError
}

// DoctorError describes a single failed check.
type DoctorError struct {
	Check string
	Err   error
}

// RenderDoctor writes a human-friendly status block.
func (r *Renderer) RenderDoctor(rep DoctorReport) error {
	w := &errWriter{w: r.w}
	header := r.st.header.Render("remetric doctor")
	w.printf("%s — %s (in %s)\n\n", header, rep.PrometheusURL, rep.Elapsed.Round(time.Millisecond))

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
	w.printf("%s tsdb stats accessible (auth: %s)\n", mark(rep.TSDBStatsOK), rep.AuthMethod)
	if rep.NumSeries > 0 {
		w.printf("     active series: %s\n", fmtInt(rep.NumSeries))
	}

	if len(rep.Errors) > 0 {
		w.printf("\nIssues:\n")
		for _, e := range rep.Errors {
			w.printf("  %s %s: %v\n", r.st.warn.Render("·"), e.Check, e.Err)
		}
	}

	return w.err
}
