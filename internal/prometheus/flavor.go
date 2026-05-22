// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package prometheus

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
)

// Flavor identifies the backend dialect a Client is talking to.
type Flavor int

const (
	// FlavorUnknown means flavor detection has not run yet.
	FlavorUnknown Flavor = iota
	// FlavorProm is upstream Prometheus.
	FlavorProm
	// FlavorVictoria is VictoriaMetrics (single-binary, cluster, or behind vmauth).
	FlavorVictoria
)

// String returns a stable lowercase label.
func (f Flavor) String() string {
	switch f {
	case FlavorProm:
		return "prometheus"
	case FlavorVictoria:
		return "victoria"
	default:
		return "unknown"
	}
}

// Flavor returns the cached detection result. Returns FlavorUnknown
// until ensureFlavor has run successfully.
func (c *Client) Flavor() Flavor {
	c.flavorMu.Lock()
	defer c.flavorMu.Unlock()
	return c.flavor
}

// FlavorHint returns the flavor hint set via WithFlavorHint, or
// FlavorUnknown if no hint was set. Useful for callers that want to
// verify which hint a Client was constructed with.
func (c *Client) FlavorHint() Flavor {
	c.flavorMu.Lock()
	defer c.flavorMu.Unlock()
	return c.flavorHint
}

// ensureFlavor lazily detects the backend dialect. The first successful
// or terminal result is cached for the lifetime of the Client. Transient
// ctx errors (Canceled, DeadlineExceeded) are NOT cached so that callers
// can retry with a fresh context. When flavorHint is set to anything other
// than FlavorUnknown, detection is skipped and the hint becomes the answer.
func (c *Client) ensureFlavor(ctx context.Context) error {
	c.flavorMu.Lock()
	defer c.flavorMu.Unlock()
	if c.flavorDone {
		return c.flavorErr
	}
	if c.flavorHint != FlavorUnknown {
		c.flavor = c.flavorHint
		c.flavorDone = true
		return nil
	}
	flavor, err := c.detectFlavor(ctx)
	if err != nil && (errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)) {
		// Transient - do not cache. Next call retries.
		return err
	}
	c.flavor = flavor
	c.flavorErr = err
	c.flavorDone = true
	return err
}

// detectFlavor probes /api/v1/status/buildinfo and classifies the
// response. Caller must not invoke directly; use ensureFlavor.
func (c *Client) detectFlavor(ctx context.Context) (Flavor, error) {
	body, err := c.do(ctx, http.MethodGet, "/api/v1/status/buildinfo", nil)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return FlavorUnknown, err
		}
		var herr *Error
		if errors.As(err, &herr) {
			switch herr.StatusCode {
			case http.StatusNotFound:
				return FlavorVictoria, nil
			case http.StatusUnauthorized, http.StatusForbidden:
				// Auth failures are not a detection-algorithm failure: surface
				// ErrAuth directly so callers can distinguish credentials from
				// ambiguous backend responses.
				return FlavorUnknown, ErrAuth
			}
		}
		return FlavorUnknown, ErrFlavorDetectFailed
	}
	var env envelope[BuildInfo]
	if err := json.Unmarshal(body, &env); err != nil {
		c.logger.Warn("buildinfo parse failed, defaulting to prometheus flavor", "err", err)
		return FlavorProm, nil
	}
	// Cache the parsed BuildInfo so callers of Client.BuildInfo() reuse this fetch.
	// Safe without a separate lock: detectFlavor runs under c.flavorMu via ensureFlavor.
	c.buildInfoCache = &env.Data
	if env.Data.Revision != "" && env.Data.GoVersion != "" {
		return FlavorProm, nil
	}
	if env.Data.Version != "" {
		return FlavorVictoria, nil
	}
	c.logger.Warn("buildinfo parse ambiguous, defaulting to prometheus flavor")
	return FlavorProm, nil
}
