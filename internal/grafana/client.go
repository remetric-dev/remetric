// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package grafana

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"reflect"
	"time"

	retryablehttp "github.com/hashicorp/go-retryablehttp"
)

// New constructs a Client. The URL must be absolute and http/https.
// Default options: anonymous, 5 concurrent requests, 30s per-request
// timeout, User-Agent="remetric/dev", logger=slog.Default().
func New(rawURL string, opts ...Option) (*Client, error) {
	c := &Client{
		auth:        noAuth{},
		timeout:     30 * time.Second,
		maxInFlight: 5,
		userAgent:   "remetric/dev",
		logger:      slog.Default(),
	}

	var firstAuthType reflect.Type
	for _, opt := range opts {
		opt(c)
		if _, isNone := c.auth.(noAuth); isNone {
			continue
		}
		t := reflect.TypeOf(c.auth)
		if firstAuthType == nil {
			firstAuthType = t
		} else if firstAuthType != t {
			return nil, fmt.Errorf("remetric: %w", ErrConflictingAuth)
		}
	}

	if rawURL == "" {
		return nil, fmt.Errorf("remetric: %w: empty url", ErrInvalidURL)
	}
	u, err := url.Parse(rawURL)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("remetric: %w: %q", ErrInvalidURL, rawURL)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return nil, fmt.Errorf("remetric: %w: scheme=%s", ErrInvalidURL, u.Scheme)
	}
	c.baseURL = u
	c.sem = make(chan struct{}, c.maxInFlight)

	rc := retryablehttp.NewClient()
	rc.RetryMax = 3
	rc.RetryWaitMin = 500 * time.Millisecond
	rc.RetryWaitMax = 8 * time.Second
	rc.HTTPClient = &http.Client{
		Timeout: c.timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: c.tlsSkipVerify}, //nolint:gosec // gated by opt-in flag
		},
	}
	rc.Logger = nil // silence retry logger; debug noise comes through slog instead
	c.hc = rc

	return c, nil
}

// do executes an HTTP request against the Grafana base URL.
func (c *Client) do(ctx context.Context, method, pathAndQuery string, body io.Reader) ([]byte, error) {
	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	u := *c.baseURL
	parsed, err := url.Parse(pathAndQuery)
	if err != nil {
		return nil, fmt.Errorf("remetric: invalid path %q: %w", pathAndQuery, err)
	}
	u.Path = singleJoin(u.Path, parsed.Path)
	u.RawQuery = parsed.RawQuery

	req, err := retryablehttp.NewRequestWithContext(ctx, method, u.String(), body)
	if err != nil {
		return nil, fmt.Errorf("remetric: build request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	c.auth.apply(req.Request)

	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("remetric: request to %s: %w", u.String(), err)
	}
	defer func() { _ = resp.Body.Close() }()

	buf, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("remetric: read body: %w", err)
	}

	if resp.StatusCode >= 400 {
		switch resp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			return nil, fmt.Errorf("remetric: grafana %s %s -> %d: %w", method, u.String(), resp.StatusCode, ErrAuth)
		case http.StatusNotFound:
			return nil, fmt.Errorf("remetric: grafana %s %s -> 404: %w", method, u.String(), ErrNotFound)
		default:
			return nil, fmt.Errorf("remetric: grafana %s %s -> %d: %s", method, u.String(), resp.StatusCode, string(buf))
		}
	}
	return buf, nil
}

// singleJoin joins URL path segments without collapsing slashes
// across two non-empty path components.
func singleJoin(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	if a[len(a)-1] == '/' && b[0] == '/' {
		return a + b[1:]
	}
	if a[len(a)-1] != '/' && b[0] != '/' {
		return a + "/" + b
	}
	return a + b
}
