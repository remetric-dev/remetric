// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package grafana

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNoAuth_AddsNothing(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example/", nil)
	noAuth{}.apply(req)
	if got := req.Header.Get("Authorization"); got != "" {
		t.Errorf("noAuth set Authorization = %q, want empty", got)
	}
}

func TestBearerAuth_AddsHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example/", nil)
	bearerAuth{token: "secret"}.apply(req)
	if got, want := req.Header.Get("Authorization"), "Bearer secret"; got != want {
		t.Errorf("Authorization = %q, want %q", got, want)
	}
}

func TestBasicAuth_AddsHeader(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "http://example/", nil)
	basicAuth{user: "u", pass: "p"}.apply(req)
	user, pass, ok := req.BasicAuth()
	if !ok || user != "u" || pass != "p" {
		t.Errorf("BasicAuth = %q:%q ok=%v, want u:p ok=true", user, pass, ok)
	}
}
