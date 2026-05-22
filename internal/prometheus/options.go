// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package prometheus

import (
	"log/slog"
	"time"
)

// Option configures a Client. Applied in order.
type Option func(*Client)

// WithBearerToken sets a bearer-token auth strategy.
func WithBearerToken(token string) Option {
	return func(c *Client) { c.auth = bearerAuth{token: token} }
}

// WithBasicAuth sets a basic-auth strategy.
func WithBasicAuth(user, pass string) Option {
	return func(c *Client) { c.auth = basicAuth{user: user, pass: pass} }
}

// WithTLSSkipVerify disables TLS verification on the transport.
// Logged as a warning by New when enabled.
func WithTLSSkipVerify(skip bool) Option {
	return func(c *Client) { c.tlsSkipVerify = skip }
}

// WithTimeout sets the per-attempt HTTP timeout (http.Client.Timeout).
// Overall operation deadlines should be enforced via the request context.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.timeout = d }
}

// WithMaxInFlight bounds concurrent outbound requests; default is 5. Values
// below 1 are clamped to 1 — a non-positive bound would deadlock semaphore
// consumers (errgroup, client.sem).
func WithMaxInFlight(n int) Option {
	return func(c *Client) {
		if n < 1 {
			n = 1
		}
		c.maxInFlight = n
	}
}

// WithUserAgent overrides the default User-Agent header.
func WithUserAgent(ua string) Option {
	return func(c *Client) { c.userAgent = ua }
}

// WithLogger installs a slog logger; required for retry decisions and warnings.
func WithLogger(l *slog.Logger) Option {
	return func(c *Client) {
		if l == nil {
			c.logger = slog.Default()
			return
		}
		c.logger = l
	}
}

// WithFlavorHint forces the backend flavor and skips detection.
// FlavorUnknown means "auto-detect" (the default).
func WithFlavorHint(f Flavor) Option {
	return func(c *Client) { c.flavorHint = f }
}
