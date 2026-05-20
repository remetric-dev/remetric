// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package prometheus

import (
	"errors"
	"fmt"
	"strings"
	"testing"
)

func TestError_Error(t *testing.T) {
	e := &Error{StatusCode: 503, Status: "503 Service Unavailable", Method: "GET", URL: "http://x/api", Body: "overloaded"}
	want := `prometheus: GET http://x/api -> 503 Service Unavailable: overloaded`
	if got := e.Error(); got != want {
		t.Errorf("Error.Error() = %q, want %q", got, want)
	}
}

func TestError_BodyTruncation(t *testing.T) {
	body := make([]byte, 2000)
	for i := range body {
		body[i] = 'a'
	}
	e := &Error{StatusCode: 500, Status: "500", Method: "GET", URL: "http://x", Body: string(body)}
	if len(e.Error()) > 1200 {
		t.Errorf("Error.Error() len = %d, want truncated under 1200", len(e.Error()))
	}
	if got := e.Error(); !strings.HasSuffix(got, "...[truncated]") {
		t.Errorf("Error.Error() did not include truncation marker; got tail %q", got[len(got)-32:])
	}
}

func TestError_Is(t *testing.T) {
	e := &Error{StatusCode: 401, sentinel: ErrAuth}
	if !errors.Is(e, ErrAuth) {
		t.Errorf("errors.Is(%v, ErrAuth) = false, want true", e)
	}
}

func TestError_Is_NoSentinel(t *testing.T) {
	e := &Error{StatusCode: 500, Status: "500", Method: "GET", URL: "http://x"}
	if errors.Is(e, ErrAuth) {
		t.Errorf("errors.Is(*Error{nil sentinel}, ErrAuth) = true, want false")
	}
}

func TestErrInvalidArgument_IsSentinel(t *testing.T) {
	err := fmt.Errorf("remetric: %w: foo", ErrInvalidArgument)
	if !errors.Is(err, ErrInvalidArgument) {
		t.Errorf("errors.Is(%v, ErrInvalidArgument) = false, want true", err)
	}
}
