// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

// Package progress surfaces per-phase scan progress to a TTY-aware writer.
// When the writer is not a terminal or the caller passed noProgress=true,
// New returns a no-op reporter so JSON/Markdown/HTML output stays unpolluted.
//
// On a real terminal New returns a live spinner that repaints in place:
//
//	▸ alerthygiene... ⠋ 12s
//
// The line is overwritten on each tick via the ANSI erase-line escape
// (\r\x1b[K). When Done is called the spinner is replaced by the final
// status line:
//
//	▸ alerthygiene... done (12.103s, 2 warnings)
package progress

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"golang.org/x/term"
)

// Reporter surfaces per-phase progress to a TTY-aware writer.
// Implementations are safe for serial Start/Done across phases on a
// single reporter; concurrent Start on a single reporter is not
// supported (the analyzer driver runs phases sequentially).
type Reporter interface {
	Start(name string)
	Done(name string, dur time.Duration, warnings int)
}

// New returns a TTY-aware Reporter writing to w. Returns a no-op when:
//   - w is nil
//   - w is not an *os.File backed by a terminal
//   - noProgress is true
func New(w io.Writer, noProgress bool) Reporter {
	return newImpl(w, noProgress, false /*forceTTY*/, true /*wantSpinner*/)
}

// newWithTTY is the test seam for the simple line reporter (no spinner).
// When forceTTY is true the TTY check is bypassed.
func newWithTTY(w io.Writer, noProgress, forceTTY bool) Reporter {
	return newImpl(w, noProgress, forceTTY, false)
}

// newWithTTYSpinner is the test seam for the spinner reporter.
// When forceTTY is true the TTY check is bypassed.
func newWithTTYSpinner(w io.Writer, noProgress, forceTTY bool) Reporter {
	return newImpl(w, noProgress, forceTTY, true)
}

func newImpl(w io.Writer, noProgress, forceTTY, wantSpinner bool) Reporter {
	if noProgress || w == nil {
		return noopReporter{}
	}
	isTTY := forceTTY
	if !isTTY {
		if f, ok := w.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
			isTTY = true
		}
	}
	if !isTTY {
		return noopReporter{}
	}
	if wantSpinner {
		return &spinnerReporter{w: w}
	}
	return &lineReporter{w: w}
}

type noopReporter struct{}

func (noopReporter) Start(string)                    {}
func (noopReporter) Done(string, time.Duration, int) {}

// lineReporter writes one status line per phase without intermediate ticks.
// Kept for tests and as a fallback if a future caller wants the simpler shape.
type lineReporter struct {
	w  io.Writer
	mu sync.Mutex
}

func (r *lineReporter) Start(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, _ = fmt.Fprintf(r.w, "▸ %s... ", name)
}

func (r *lineReporter) Done(_ string, dur time.Duration, warnings int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, _ = fmt.Fprint(r.w, formatDoneTail(dur, warnings))
}

// spinnerReporter shows a braille spinner with elapsed seconds while a
// phase runs, then overwrites with the final done line.
type spinnerReporter struct {
	w    io.Writer
	mu   sync.Mutex
	stop chan struct{}
	done chan struct{}
}

const (
	spinnerFrames   = "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏"
	spinnerInterval = 100 * time.Millisecond
	eraseLine       = "\r\x1b[K"
)

func (r *spinnerReporter) Start(name string) {
	r.mu.Lock()
	if r.stop != nil {
		// Defensive: previous Done not called. Stop the prior goroutine
		// before kicking a new one so we never leak goroutines.
		close(r.stop)
		ch := r.done
		r.stop = nil
		r.done = nil
		r.mu.Unlock()
		<-ch
		r.mu.Lock()
	}
	r.stop = make(chan struct{})
	r.done = make(chan struct{})
	stop := r.stop
	done := r.done
	r.mu.Unlock()
	start := time.Now()
	go r.spin(name, start, stop, done)
}

func (r *spinnerReporter) spin(name string, start time.Time, stop, done chan struct{}) {
	defer close(done)
	frames := []rune(spinnerFrames)
	r.paint(name, frames[0], time.Since(start))
	ticker := time.NewTicker(spinnerInterval)
	defer ticker.Stop()
	i := 0
	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			i++
			r.paint(name, frames[i%len(frames)], time.Since(start))
		}
	}
}

func (r *spinnerReporter) paint(name string, frame rune, elapsed time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, _ = fmt.Fprintf(r.w, "%s▸ %s... %c %s", eraseLine, name, frame, elapsed.Round(time.Second))
}

func (r *spinnerReporter) Done(name string, dur time.Duration, warnings int) {
	r.mu.Lock()
	if r.stop != nil {
		close(r.stop)
		ch := r.done
		r.stop = nil
		r.done = nil
		r.mu.Unlock()
		<-ch
		r.mu.Lock()
	}
	defer r.mu.Unlock()
	_, _ = fmt.Fprintf(r.w, "%s▸ %s... %s", eraseLine, name, formatDoneTail(dur, warnings))
}

// formatDoneTail returns the trailing part of the done line. Examples:
//
//	"done (812ms)\n"
//	"done (287ms, 1 warning)\n"
//	"done (2.103s, 3 warnings)\n"
func formatDoneTail(dur time.Duration, warnings int) string {
	rounded := dur.Round(time.Millisecond)
	if warnings == 0 {
		return fmt.Sprintf("done (%s)\n", rounded)
	}
	word := "warnings"
	if warnings == 1 {
		word = "warning"
	}
	return fmt.Sprintf("done (%s, %d %s)\n", rounded, warnings, word)
}
