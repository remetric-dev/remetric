// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package grafana

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNew_RejectsEmptyURL(t *testing.T) {
	_, err := New("")
	if !errors.Is(err, ErrInvalidURL) {
		t.Errorf("New(\"\") error = %v, want ErrInvalidURL", err)
	}
}

func TestNew_RejectsNonHTTPScheme(t *testing.T) {
	_, err := New("ftp://example.com")
	if !errors.Is(err, ErrInvalidURL) {
		t.Errorf("New(ftp) error = %v, want ErrInvalidURL", err)
	}
}

func TestNew_ConflictingAuth(t *testing.T) {
	_, err := New("http://example.com", WithBearerToken("a"), WithBasicAuth("u", "p"))
	if !errors.Is(err, ErrConflictingAuth) {
		t.Errorf("error = %v, want ErrConflictingAuth", err)
	}
}

func TestNew_SameKindAuthAllowed(t *testing.T) {
	_, err := New("http://example.com", WithBearerToken("a"), WithBearerToken("b"))
	if err != nil {
		t.Errorf("same-kind auth should be allowed, got %v", err)
	}
}

func TestDo_AppliesBearerToken(t *testing.T) {
	var captured string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`ok`))
	}))
	defer ts.Close()
	c, err := New(ts.URL, WithBearerToken("secret"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.do(context.Background(), http.MethodGet, "/", nil); err != nil {
		t.Fatalf("do: %v", err)
	}
	if want := "Bearer secret"; captured != want {
		t.Errorf("Authorization = %q, want %q", captured, want)
	}
}

func TestDo_ReturnsErrAuthOn401(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`unauthorized`))
	}))
	defer ts.Close()
	c, _ := New(ts.URL)
	_, err := c.do(context.Background(), http.MethodGet, "/foo", nil)
	if !errors.Is(err, ErrAuth) {
		t.Errorf("err = %v, want ErrAuth", err)
	}
}

func TestDo_ReturnsErrNotFoundOn404(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()
	c, _ := New(ts.URL)
	_, err := c.do(context.Background(), http.MethodGet, "/foo", nil)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("err = %v, want ErrNotFound", err)
	}
}

func TestClient_BaseURL_ReturnsAbsoluteCopy(t *testing.T) {
	c, err := New("https://grafana.example.com/sub/")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	u := c.BaseURL()
	if u == nil {
		t.Fatal("BaseURL returned nil")
	}
	if u.Scheme != "https" || u.Host != "grafana.example.com" || u.Path != "/sub/" {
		t.Errorf("BaseURL = %+v, want https://grafana.example.com/sub/", u)
	}
	// Caller mutation must not affect the client.
	u.Path = "/mutated"
	again := c.BaseURL()
	if again.Path != "/sub/" {
		t.Errorf("BaseURL after caller mutation = %q, want /sub/ (defensive copy)", again.Path)
	}
}
