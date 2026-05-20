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

// WithTimeout caps the duration of a single HTTP request (incl. retries).
func WithTimeout(d time.Duration) Option {
	return func(c *Client) { c.timeout = d }
}

// WithMaxInFlight bounds concurrent outbound requests; default is 5.
func WithMaxInFlight(n int) Option {
	return func(c *Client) { c.maxInFlight = n }
}

// WithUserAgent overrides the default User-Agent header.
func WithUserAgent(ua string) Option {
	return func(c *Client) { c.userAgent = ua }
}

// WithLogger installs a slog logger; required for retry decisions and warnings.
func WithLogger(l *slog.Logger) Option {
	return func(c *Client) { c.logger = l }
}
