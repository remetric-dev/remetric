package prometheus

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"time"

	retryablehttp "github.com/hashicorp/go-retryablehttp"
)

// Client is a bounded-concurrency Prometheus HTTP client.
type Client struct {
	baseURL *url.URL
	hc      *retryablehttp.Client

	auth          auth
	tlsSkipVerify bool
	timeout       time.Duration
	maxInFlight   int
	userAgent     string
	logger        *slog.Logger

	sem chan struct{}
}

// New constructs a Client. The URL must be absolute and use http/https.
// Default options: noAuth, 5 concurrent requests, 30s per request timeout,
// User-Agent="remetric/dev", logger=slog.Default().
func New(rawURL string, opts ...Option) (*Client, error) {
	c := &Client{
		auth:        noAuth{},
		timeout:     30 * time.Second,
		maxInFlight: 5,
		userAgent:   "remetric/dev",
		logger:      slog.Default(),
	}

	hadAuth := false
	for _, opt := range opts {
		prev := c.auth
		opt(c)
		if _, isNone := c.auth.(noAuth); !isNone && prev != c.auth {
			if hadAuth {
				return nil, fmt.Errorf("remetric: %w", ErrConflictingAuth)
			}
			if _, prevWasNone := prev.(noAuth); !prevWasNone {
				return nil, fmt.Errorf("remetric: %w", ErrConflictingAuth)
			}
			hadAuth = true
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
	rc.Logger = slogLogger{c.logger}
	rc.RetryMax = 3
	rc.RetryWaitMin = 500 * time.Millisecond
	rc.RetryWaitMax = 8 * time.Second
	rc.HTTPClient = &http.Client{
		Timeout: c.timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: c.tlsSkipVerify}, //nolint:gosec // gated by opt-in flag
		},
	}
	if c.tlsSkipVerify {
		c.logger.Warn("TLS verification disabled for Prometheus client", "url", u.Redacted())
	}
	c.hc = rc

	return c, nil
}

// slogLogger adapts *slog.Logger to retryablehttp.LeveledLogger.
type slogLogger struct{ *slog.Logger }

func (s slogLogger) Error(msg string, kv ...interface{}) { s.Logger.Error(msg, kv...) }
func (s slogLogger) Warn(msg string, kv ...interface{})  { s.Logger.Warn(msg, kv...) }
func (s slogLogger) Info(msg string, kv ...interface{})  { s.Logger.Info(msg, kv...) }
func (s slogLogger) Debug(msg string, kv ...interface{}) { s.Logger.Debug(msg, kv...) }

// do executes an HTTP request against the Prometheus base URL, honouring
// auth, user-agent, max-in-flight, retries, and ctx.
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

	start := time.Now()
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("remetric: request to %s: %w", u.String(), err)
	}
	defer func() { _ = resp.Body.Close() }()

	buf, err := io.ReadAll(io.LimitReader(resp.Body, 16*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("remetric: read body: %w", err)
	}

	c.logger.Debug("prometheus request",
		"method", method,
		"url", u.String(),
		"status", resp.StatusCode,
		"duration", time.Since(start).String(),
		"bytes", len(buf))

	if resp.StatusCode >= 400 {
		e := &Error{StatusCode: resp.StatusCode, Status: resp.Status, Method: method, URL: u.String(), Body: string(buf)}
		switch resp.StatusCode {
		case http.StatusUnauthorized, http.StatusForbidden:
			e.sentinel = ErrAuth
		case http.StatusNotFound:
			e.sentinel = ErrNotFound
		}
		return nil, e
	}
	return buf, nil
}

// singleJoin joins URL path segments collapsing duplicate slashes.
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
