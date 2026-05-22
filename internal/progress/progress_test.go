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
		t.Errorf("buf = %q, want singular '1 warning'", got)
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
		t.Errorf("buf = %q, want plural '3 warnings'", got)
	}
}

func TestLineReporter_DoneRoundsDurationToMillisecond(t *testing.T) {
	tests := []struct {
		name string
		dur  time.Duration
		want string
	}{
		{"sub-ms truncated", 103*time.Millisecond + 624*time.Microsecond + 458*time.Nanosecond, "done (104ms)\n"},
		{"tiny duration", 1*time.Millisecond + 811*time.Microsecond, "done (2ms)\n"},
		{"super-second", 2*time.Second + 103*time.Millisecond + 87*time.Microsecond, "done (2.103s)\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			r := newWithTTY(&buf, false, true)
			r.Start("phase")
			r.Done("phase", tc.dur, 0)
			if got := buf.String(); !strings.Contains(got, tc.want) {
				t.Errorf("buf = %q, want substring %q", got, tc.want)
			}
		})
	}
}

func TestLineReporter_NoProgressOverridesTTY(t *testing.T) {
	var buf bytes.Buffer
	r := newWithTTY(&buf, true, true)
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
}

type safeBuf struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuf) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuf) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func finalAfterRedraws(s string) string {
	const eraseLine = "\r\x1b[K"
	idx := strings.LastIndex(s, eraseLine)
	if idx < 0 {
		return s
	}
	return s[idx+len(eraseLine):]
}

func TestSpinner_NoProgressYieldsNoop(t *testing.T) {
	var buf bytes.Buffer
	r := newWithTTYSpinner(&buf, true, true)
	r.Start("phase")
	r.Done("phase", time.Second, 0)
	if got := buf.String(); got != "" {
		t.Errorf("noProgress=true should suppress output: buf = %q", got)
	}
}

func TestSpinner_NonTTYWriterYieldsNoop(t *testing.T) {
	var buf bytes.Buffer
	r := newWithTTYSpinner(&buf, false, false)
	r.Start("phase")
	r.Done("phase", time.Second, 0)
	if got := buf.String(); got != "" {
		t.Errorf("non-TTY writer with spinner factory must be noop: buf = %q", got)
	}
}

func TestSpinner_FinalLineMatchesLineReporter(t *testing.T) {
	var buf safeBuf
	r := newWithTTYSpinner(&buf, false, true)
	r.Start("cardinality")
	r.Done("cardinality", 812*time.Millisecond, 0)
	want := "▸ cardinality... done (812ms)\n"
	if got := finalAfterRedraws(buf.String()); got != want {
		t.Errorf("final segment = %q, want %q", got, want)
	}
}

func TestSpinner_FinalLineSingularPluralWarnings(t *testing.T) {
	tests := []struct {
		name     string
		warnings int
		want     string
	}{
		{"singular", 1, "▸ unusedmetrics... done (287ms, 1 warning)\n"},
		{"plural", 3, "▸ unusedmetrics... done (287ms, 3 warnings)\n"},
		{"none", 0, "▸ unusedmetrics... done (287ms)\n"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf safeBuf
			r := newWithTTYSpinner(&buf, false, true)
			r.Start("unusedmetrics")
			r.Done("unusedmetrics", 287*time.Millisecond, tc.warnings)
			if got := finalAfterRedraws(buf.String()); got != tc.want {
				t.Errorf("final = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSpinner_EmitsFramesBetweenStartAndDone(t *testing.T) {
	var buf safeBuf
	r := newWithTTYSpinner(&buf, false, true)
	r.Start("phase")
	time.Sleep(250 * time.Millisecond)
	r.Done("phase", 250*time.Millisecond, 0)
	s := buf.String()
	if !strings.ContainsAny(s, "⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏") {
		t.Errorf("expected at least one spinner frame, got: %q", s)
	}
	if !strings.Contains(finalAfterRedraws(s), "done (250ms)") {
		t.Errorf("final must contain 'done (250ms)', got: %q", finalAfterRedraws(s))
	}
}

func TestSpinner_DoneStopsSpinnerGoroutine(t *testing.T) {
	var buf safeBuf
	r := newWithTTYSpinner(&buf, false, true)
	r.Start("phase")
	time.Sleep(150 * time.Millisecond)
	r.Done("phase", 150*time.Millisecond, 0)
	snapshot := len(buf.String())
	time.Sleep(250 * time.Millisecond)
	if got := len(buf.String()); got != snapshot {
		t.Errorf("buffer grew after Done: snapshot=%d, later=%d", snapshot, got)
	}
}

func TestSpinner_SequentialPhases(t *testing.T) {
	var buf safeBuf
	r := newWithTTYSpinner(&buf, false, true)
	for _, name := range []string{"a", "b", "c", "d"} {
		r.Start(name)
		time.Sleep(120 * time.Millisecond)
		r.Done(name, 120*time.Millisecond, 0)
	}
	s := buf.String()
	for _, name := range []string{"a", "b", "c", "d"} {
		if !strings.Contains(s, "done") || !strings.Contains(s, name) {
			t.Errorf("expected phase %q to render done line: %q", name, s)
		}
	}
}
