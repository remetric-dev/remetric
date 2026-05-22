// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Dashboard is the minimal parsed representation of /api/dashboards/uid/{uid}.
type Dashboard struct {
	UID    string  `json:"uid"`
	Title  string  `json:"title"`
	Panels []panel `json:"panels"`
}

type panel struct {
	Type    string   `json:"type"`
	Panels  []panel  `json:"panels,omitempty"`
	Targets []target `json:"targets,omitempty"`
}

type target struct {
	Expr       string     `json:"expr"`
	Datasource datasource `json:"datasource"`
}

// datasource handles two shapes Grafana emits:
//  1. {"datasource": "Prometheus"}      - legacy string
//  2. {"datasource": {"type": "prometheus", "uid": "..."}} - modern
//
// Both unmarshal into the same struct. Type defaults to "" for the
// legacy string form; that case is treated as Prometheus.
type datasource struct {
	Type string `json:"type"`
}

func (d *datasource) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		// Legacy string datasource - treat as Prometheus by default.
		d.Type = "prometheus"
		return nil
	}
	type aux datasource
	return json.Unmarshal(b, (*aux)(d))
}

// Dashboard fetches and parses a dashboard by UID.
func (c *Client) Dashboard(ctx context.Context, uid string) (*Dashboard, error) {
	if uid == "" {
		return nil, fmt.Errorf("grafana: %w: empty uid", ErrInvalidArgument)
	}
	body, err := c.do(ctx, http.MethodGet, "/api/dashboards/uid/"+uid, nil)
	if err != nil {
		return nil, err
	}
	var env struct {
		Dashboard Dashboard `json:"dashboard"`
	}
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("grafana: parse dashboard %q: %w", uid, err)
	}
	return &env.Dashboard, nil
}

// Queries returns every PromQL expression referenced by Prometheus
// targets across the dashboard, walking rows recursively. Empty
// expressions and non-Prometheus targets are skipped.
func (d *Dashboard) Queries() []string {
	var out []string
	collectPanelQueries(d.Panels, &out)
	return out
}

func collectPanelQueries(panels []panel, out *[]string) {
	for _, p := range panels {
		collectPanelQueries(p.Panels, out)
		for _, t := range p.Targets {
			if t.Datasource.Type != "" && t.Datasource.Type != "prometheus" {
				continue
			}
			if t.Expr == "" {
				continue
			}
			*out = append(*out, t.Expr)
		}
	}
}
