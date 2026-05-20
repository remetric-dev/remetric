package terminal

import (
	"bytes"
	"testing"
)

func TestRenderer_NoColorViaOption(t *testing.T) {
	r := New(&bytes.Buffer{}, WithColor(false))
	if !r.noColor {
		t.Errorf("noColor = false, want true")
	}
}

func TestRenderer_AutoNoColorOnNonTTY(t *testing.T) {
	r := New(&bytes.Buffer{})
	if !r.noColor {
		t.Errorf("noColor for buffer writer = false, want true (non-TTY)")
	}
}

func TestRenderer_RespectsNoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	r := New(&bytes.Buffer{}, WithColor(true))
	if !r.noColor {
		t.Errorf("with NO_COLOR=1, noColor = false, want true")
	}
}
