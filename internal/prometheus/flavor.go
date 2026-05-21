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
func (c *Client) Flavor() Flavor { return c.flavor }

// ensureFlavor lazily detects the backend dialect. The first successful
// or terminal result is cached for the lifetime of the Client. When
// flavorHint is set to anything other than FlavorUnknown, detection is
// skipped and the hint becomes the answer.
func (c *Client) ensureFlavor(ctx context.Context) error {
	c.flavorOnce.Do(func() {
		if c.flavorHint != FlavorUnknown {
			c.flavor = c.flavorHint
			return
		}
		c.flavor, c.flavorErr = c.detectFlavor(ctx)
	})
	return c.flavorErr
}

// detectFlavor probes /api/v1/status/buildinfo and classifies the
// response. Caller must not invoke directly; use ensureFlavor.
func (c *Client) detectFlavor(ctx context.Context) (Flavor, error) {
	body, err := c.do(ctx, http.MethodGet, "/api/v1/status/buildinfo", nil)
	if err != nil {
		var herr *Error
		if errors.As(err, &herr) {
			switch herr.StatusCode {
			case http.StatusNotFound:
				return FlavorVictoria, nil
			case http.StatusUnauthorized, http.StatusForbidden:
				return FlavorUnknown, ErrFlavorDetectFailed
			}
		}
		return FlavorUnknown, ErrFlavorDetectFailed
	}
	var env envelope[map[string]string]
	if err := json.Unmarshal(body, &env); err != nil {
		c.logger.Warn("buildinfo parse failed, defaulting to prometheus flavor", "err", err)
		return FlavorProm, nil
	}
	hasRev := env.Data["revision"] != ""
	hasGo := env.Data["goVersion"] != ""
	if hasRev && hasGo {
		return FlavorProm, nil
	}
	if env.Data["version"] != "" {
		return FlavorVictoria, nil
	}
	c.logger.Warn("buildinfo parse ambiguous, defaulting to prometheus flavor")
	return FlavorProm, nil
}
