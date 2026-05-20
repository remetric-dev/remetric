package prometheus

import (
	"crypto/tls"
	"fmt"
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
