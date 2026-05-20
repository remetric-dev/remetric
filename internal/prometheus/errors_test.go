package prometheus

import (
	"errors"
	"testing"
)

func TestError_Error(t *testing.T) {
	e := &Error{StatusCode: 503, Status: "503 Service Unavailable", URL: "http://x/api", Body: "overloaded"}
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
	e := &Error{StatusCode: 500, Status: "500", URL: "http://x", Body: string(body)}
	if len(e.Error()) > 1200 {
		t.Errorf("Error.Error() len = %d, want truncated under 1200", len(e.Error()))
	}
}

func TestError_Is(t *testing.T) {
	e := &Error{StatusCode: 401, sentinel: ErrAuth}
	if !errors.Is(e, ErrAuth) {
		t.Errorf("errors.Is(%v, ErrAuth) = false, want true", e)
	}
}
