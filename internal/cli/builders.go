// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package cli

import (
	"fmt"
	"strings"

	"github.com/remetric-dev/remetric/internal/config"
	prom "github.com/remetric-dev/remetric/internal/prometheus"
)

// buildVMAlertClient returns a Prometheus-protocol client pointed at the
// vmalert URL configured in cfg, or (nil, nil) if no URL is set. Auth
// flags default to vmalert-specific options; if neither vmalert-token nor
// vmalert-basic-auth is set, the client inherits the Prometheus auth
// (common case for vmauth-fronted clusters).
//
// The returned client carries WithFlavorHint(FlavorProm) since vmalert
// speaks the Prometheus rules-API dialect - no detection round-trip needed.
func buildVMAlertClient(cfg *config.Config, ua string) (*prom.Client, error) {
	if cfg.VMAlert.URL == "" {
		return nil, nil
	}
	if cfg.VMAlert.Token != "" && cfg.VMAlert.BasicAuth != "" {
		return nil, fmt.Errorf("vmalert: %w", prom.ErrConflictingAuth)
	}
	opts := []prom.Option{
		prom.WithUserAgent(ua),
		prom.WithFlavorHint(prom.FlavorProm),
	}
	switch {
	case cfg.VMAlert.Token != "":
		opts = append(opts, prom.WithBearerToken(cfg.VMAlert.Token))
	case cfg.VMAlert.BasicAuth != "":
		u, p, ok := splitBasicAuth(cfg.VMAlert.BasicAuth)
		if !ok {
			return nil, fmt.Errorf("vmalert-basic-auth: expected user:password")
		}
		opts = append(opts, prom.WithBasicAuth(u, p))
	case cfg.Prometheus.Token != "":
		opts = append(opts, prom.WithBearerToken(cfg.Prometheus.Token))
	case cfg.Prometheus.BasicAuth != "":
		u, p, ok := splitBasicAuth(cfg.Prometheus.BasicAuth)
		if !ok {
			return nil, fmt.Errorf("prom-basic-auth: expected user:password")
		}
		opts = append(opts, prom.WithBasicAuth(u, p))
	}
	if cfg.VMAlert.TLSSkipVerify || cfg.Prometheus.TLSSkipVerify {
		opts = append(opts, prom.WithTLSSkipVerify(true))
	}
	if cfg.Timeout > 0 {
		opts = append(opts, prom.WithTimeout(cfg.Timeout))
	}
	return prom.New(cfg.VMAlert.URL, opts...)
}

// splitBasicAuth parses "user:password". Returns ok=false on missing colon
// or empty input. Password portions may contain additional colons.
func splitBasicAuth(s string) (user, pass string, ok bool) {
	i := strings.IndexByte(s, ':')
	if i < 0 {
		return "", "", false
	}
	return s[:i], s[i+1:], true
}
