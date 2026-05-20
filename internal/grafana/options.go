// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package grafana

import (
	"log/slog"
	"net/url"
	"time"

	retryablehttp "github.com/hashicorp/go-retryablehttp"
)

// Client is a Grafana HTTP client. Methods are implemented in client.go
// (added by Task 2.2); this file declares the struct so Option closures
// have something to populate.
type Client struct {
	baseURL *url.URL              //nolint:unused // populated by Task 2.2 (client.go: New)
	hc      *retryablehttp.Client //nolint:unused // populated by Task 2.2 (client.go: New)

	auth          auth
	tlsSkipVerify bool
	timeout       time.Duration
	maxInFlight   int
	userAgent     string
	logger        *slog.Logger //nolint:unused // populated by Task 2.2 (client.go: New)

	sem chan struct{} //nolint:unused // populated by Task 2.2 (client.go: New)
}

// Option configures a Client during construction.
type Option func(*Client)

// WithBearerToken installs a bearer-token (Grafana service-account
// API key) auth strategy.
func WithBearerToken(token string) Option {
	return func(c *Client) { c.auth = bearerAuth{token: token} }
}

// WithBasicAuth installs HTTP basic auth.
func WithBasicAuth(user, pass string) Option {
	return func(c *Client) { c.auth = basicAuth{user: user, pass: pass} }
}

// WithTLSSkipVerify disables TLS certificate verification. Use only
// against trusted networks (e.g. self-signed internal Grafana).
func WithTLSSkipVerify(skip bool) Option {
	return func(c *Client) { c.tlsSkipVerify = skip }
}

// WithTimeout sets the per-request timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.timeout = d }
}

// WithMaxInFlight caps concurrent in-flight HTTP requests.
func WithMaxInFlight(n int) Option {
	return func(c *Client) { c.maxInFlight = n }
}

// WithUserAgent sets the User-Agent header value.
func WithUserAgent(ua string) Option {
	return func(c *Client) { c.userAgent = ua }
}
