// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

// Package progress surfaces per-phase scan progress to a TTY-aware writer.
// When the writer is not a terminal or the caller passed noProgress=true,
// New returns a no-op reporter so JSON/Markdown/HTML output stays unpolluted.
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
// Implementations are safe for concurrent use; today's callers are serial,
// but races shouldn't be possible if that changes.
type Reporter interface {
	Start(name string)
	Done(name string, dur time.Duration, warnings int)
}

// New returns a TTY-aware Reporter writing to w. Returns a no-op when:
//   - w is nil
//   - w is not an *os.File backed by a terminal
//   - noProgress is true
func New(w io.Writer, noProgress bool) Reporter {
	return newWithTTY(w, noProgress, false)
}

// newWithTTY is the test seam. When forceTTY is true, the TTY check is
// bypassed and the line reporter is returned (still respecting noProgress).
func newWithTTY(w io.Writer, noProgress, forceTTY bool) Reporter {
	if noProgress || w == nil {
		return noopReporter{}
	}
	if forceTTY {
		return &lineReporter{w: w}
	}
	f, ok := w.(*os.File)
	if !ok {
		return noopReporter{}
	}
	if !term.IsTerminal(int(f.Fd())) {
		return noopReporter{}
	}
	return &lineReporter{w: w}
}

type noopReporter struct{}

func (noopReporter) Start(string)                    {}
func (noopReporter) Done(string, time.Duration, int) {}

type lineReporter struct {
	w  io.Writer
	mu sync.Mutex
}

func (r *lineReporter) Start(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, _ = fmt.Fprintf(r.w, "▸ %s... ", name)
}

func (r *lineReporter) Done(name string, dur time.Duration, warnings int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	rounded := dur.Round(time.Millisecond)
	if warnings == 0 {
		_, _ = fmt.Fprintf(r.w, "done (%s)\n", rounded)
		return
	}
	word := "warnings"
	if warnings == 1 {
		word = "warning"
	}
	_, _ = fmt.Fprintf(r.w, "done (%s, %d %s)\n", rounded, warnings, word)
}
