package prometheus

import (
	"errors"
	"testing"
)

func TestNew_DefaultsApplied(t *testing.T) {
	c, err := New("http://prom:9090")
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if c.maxInFlight != 5 {
		t.Errorf("default maxInFlight = %d, want 5", c.maxInFlight)
	}
	if c.timeout == 0 {
		t.Errorf("default timeout = 0, want > 0")
	}
	if c.userAgent == "" {
		t.Errorf("default userAgent empty")
	}
	if _, ok := c.auth.(noAuth); !ok {
		t.Errorf("default auth = %T, want noAuth", c.auth)
	}
	if c.logger == nil {
		t.Errorf("default logger nil; New should install slog.Default")
	}
}

func TestNew_RejectsInvalidURL(t *testing.T) {
	_, err := New("://not-a-url")
	if !errors.Is(err, ErrInvalidURL) {
		t.Errorf("New(bad-url) err = %v, want wrap ErrInvalidURL", err)
	}
}

func TestNew_RejectsEmptyURL(t *testing.T) {
	_, err := New("")
	if !errors.Is(err, ErrInvalidURL) {
		t.Errorf("New(empty) err = %v, want wrap ErrInvalidURL", err)
	}
}

func TestNew_RejectsConflictingAuth(t *testing.T) {
	_, err := New("http://prom:9090", WithBearerToken("t"), WithBasicAuth("u", "p"))
	if !errors.Is(err, ErrConflictingAuth) {
		t.Errorf("New(both auth) err = %v, want wrap ErrConflictingAuth", err)
	}
}
