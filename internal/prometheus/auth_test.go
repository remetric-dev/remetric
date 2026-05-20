package prometheus

import (
	"net/http"
	"testing"
)

func TestBearerAuth_Apply(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "http://x", nil)
	a := bearerAuth{token: "abc"}
	a.apply(r)
	if got, want := r.Header.Get("Authorization"), "Bearer abc"; got != want {
		t.Errorf("Authorization = %q, want %q", got, want)
	}
}

func TestBasicAuth_Apply(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "http://x", nil)
	a := basicAuth{user: "u", pass: "p"}
	a.apply(r)
	user, pass, ok := r.BasicAuth()
	if !ok || user != "u" || pass != "p" {
		t.Errorf("BasicAuth = (%q,%q,%v), want (u,p,true)", user, pass, ok)
	}
}

func TestNoAuth_Apply(t *testing.T) {
	r, _ := http.NewRequest(http.MethodGet, "http://x", nil)
	noAuth{}.apply(r)
	if got := r.Header.Get("Authorization"); got != "" {
		t.Errorf("Authorization = %q, want empty", got)
	}
}
