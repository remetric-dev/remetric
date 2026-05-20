package prometheus

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
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

func TestClient_SendsUserAgent(t *testing.T) {
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("User-Agent")
		w.WriteHeader(200)
	}))
	defer srv.Close()
	c, err := New(srv.URL, WithUserAgent("remetric/test"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if _, err := c.do(context.Background(), http.MethodGet, "/healthy", nil); err != nil {
		t.Fatalf("do: %v", err)
	}
	if seen != "remetric/test" {
		t.Errorf("User-Agent = %q, want remetric/test", seen)
	}
}

func TestClient_AppliesBearer(t *testing.T) {
	var seen string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("Authorization")
		w.WriteHeader(200)
	}))
	defer srv.Close()
	c, _ := New(srv.URL, WithBearerToken("XYZ"))
	if _, err := c.do(context.Background(), http.MethodGet, "/healthy", nil); err != nil {
		t.Fatalf("do: %v", err)
	}
	if seen != "Bearer XYZ" {
		t.Errorf("Authorization = %q, want Bearer XYZ", seen)
	}
}

func TestClient_MaxInFlightEnforced(t *testing.T) {
	var inFlight int32
	var peak int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&inFlight, 1)
		defer atomic.AddInt32(&inFlight, -1)
		for {
			if p := atomic.LoadInt32(&peak); p >= n || atomic.CompareAndSwapInt32(&peak, p, n) {
				break
			}
		}
		time.Sleep(40 * time.Millisecond)
		w.WriteHeader(200)
	}))
	defer srv.Close()
	c, _ := New(srv.URL, WithMaxInFlight(2))
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := c.do(context.Background(), http.MethodGet, "/healthy", nil); err != nil {
				t.Errorf("do: %v", err)
			}
		}()
	}
	wg.Wait()
	if atomic.LoadInt32(&peak) > 2 {
		t.Errorf("observed concurrency peak = %d, want <= 2", peak)
	}
}

func TestClient_ContextCancellation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(500 * time.Millisecond)
	}))
	defer srv.Close()
	c, _ := New(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	_, err := c.do(ctx, http.MethodGet, "/slow", nil)
	if err == nil {
		t.Errorf("do() err = nil, want context deadline exceeded")
	}
}
