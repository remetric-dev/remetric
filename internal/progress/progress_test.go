// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package progress

import (
	"bytes"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNoop_NonTTYWriter(t *testing.T) {
	var buf bytes.Buffer
	r := New(&buf, false)
	r.Start("cardinality")
	r.Done("cardinality", 800*time.Millisecond, 0)
	if got := buf.String(); got != "" {
		t.Errorf("non-TTY writer: buf = %q, want empty", got)
	}
}

func TestLineReporter_StartDoneNoWarnings(t *testing.T) {
	var buf bytes.Buffer
	r := newWithTTY(&buf, false, true)
	r.Start("cardinality")
	r.Done("cardinality", 812*time.Millisecond, 0)
	want := "▸ cardinality... done (812ms)\n"
	if got := buf.String(); got != want {
		t.Errorf("buf = %q, want %q", got, want)
	}
}

func TestLineReporter_DoneSingularWarning(t *testing.T) {
	var buf bytes.Buffer
	r := newWithTTY(&buf, false, true)
	r.Start("unusedmetrics")
	r.Done("unusedmetrics", 287*time.Millisecond, 1)
	if got := buf.String(); !strings.Contains(got, "1 warning)") {
		t.Errorf("buf = %q, want singular ‘1 warning’", got)
	}
	if got := buf.String(); strings.Contains(got, "1 warnings") {
		t.Errorf("buf = %q, should not pluralise on count=1", got)
	}
}

func TestLineReporter_DonePluralWarnings(t *testing.T) {
	var buf bytes.Buffer
	r := newWithTTY(&buf, false, true)
	r.Start("alerthygiene")
	r.Done("alerthygiene", 2*time.Second, 3)
	if got := buf.String(); !strings.Contains(got, "3 warnings)") {
		t.Errorf("buf = %q, want plural ‘3 warnings’", got)
	}
}

func TestLineReporter_NoProgressOverridesTTY(t *testing.T) {
	var buf bytes.Buffer
	r := newWithTTY(&buf, true, true) // noProgress=true wins
	r.Start("cardinality")
	r.Done("cardinality", time.Second, 0)
	if got := buf.String(); got != "" {
		t.Errorf("noProgress=true should suppress output: buf = %q", got)
	}
}

func TestLineReporter_ConcurrencySafe(t *testing.T) {
	var buf bytes.Buffer
	r := newWithTTY(&buf, false, true)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			r.Start("phase")
			r.Done("phase", time.Millisecond, 0)
		}()
	}
	wg.Wait()
	// No assertion on content (output is interleaved); the value is the
	// -race detector passing.
}
