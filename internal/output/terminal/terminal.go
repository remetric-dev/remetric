package terminal

import (
	"io"
	"os"

	"golang.org/x/term"
)

// Renderer formats findings and doctor reports for the terminal.
type Renderer struct {
	w       io.Writer
	noColor bool
	width   int
	st      styles
}

// Option configures a Renderer.
type Option func(*Renderer)

// WithColor explicitly enables or disables color. Auto-detected otherwise.
// NO_COLOR env always wins.
func WithColor(enabled bool) Option {
	return func(r *Renderer) { r.noColor = !enabled }
}

// WithWidth overrides the auto-detected terminal width.
func WithWidth(cols int) Option { return func(r *Renderer) { r.width = cols } }

// New constructs a Renderer. Color and width are auto-detected from w if
// w is *os.File; otherwise color is off, width defaults to 100.
func New(w io.Writer, opts ...Option) *Renderer {
	r := &Renderer{w: w, width: 100}

	if f, ok := w.(*os.File); ok && term.IsTerminal(int(f.Fd())) {
		r.noColor = false
		if cols, _, err := term.GetSize(int(f.Fd())); err == nil && cols > 0 {
			r.width = cols
		}
	} else {
		r.noColor = true
	}

	for _, opt := range opts {
		opt(r)
	}
	if os.Getenv("NO_COLOR") != "" {
		r.noColor = true
	}
	r.st = newStyles(r.noColor)
	return r
}
