// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package grafana

import (
	"errors"
	"fmt"
	"testing"
)

func TestSentinels_AreDistinct(t *testing.T) {
	if errors.Is(ErrInvalidURL, ErrAuth) {
		t.Errorf("ErrInvalidURL and ErrAuth are the same sentinel")
	}
	if errors.Is(ErrAuth, ErrNotFound) {
		t.Errorf("ErrAuth and ErrNotFound are the same sentinel")
	}
}

func TestSentinels_WrapAndUnwrap(t *testing.T) {
	for _, s := range []error{ErrInvalidURL, ErrAuth, ErrNotFound, ErrInvalidArgument, ErrConflictingAuth} {
		wrapped := fmt.Errorf("remetric: %w: foo", s)
		if !errors.Is(wrapped, s) {
			t.Errorf("errors.Is wrapped(%v) returned false", s)
		}
	}
}
