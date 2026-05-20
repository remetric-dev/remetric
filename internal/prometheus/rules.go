// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// Rules is the parsed /api/v1/rules response.
type Rules struct {
	Groups []RuleGroup `json:"groups"`
}

// RuleGroup is one group of rules from the response.
type RuleGroup struct {
	Name  string `json:"name"`
	File  string `json:"file"`
	Rules []Rule `json:"rules"`
}

// Rule is one alerting or recording rule. Type is either "alerting"
// or "recording". For recording rules, Name is the output metric
// name (the `record:` field); for alerting rules it is the alert
// name.
type Rule struct {
	Name  string `json:"name"`
	Query string `json:"query"`
	Type  string `json:"type"`
}

// Rules fetches and parses /api/v1/rules.
func (c *Client) Rules(ctx context.Context) (*Rules, error) {
	body, err := c.do(ctx, http.MethodGet, "/api/v1/rules", nil)
	if err != nil {
		return nil, err
	}
	var env envelope[Rules]
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("remetric: parse rules: %w", err)
	}
	return &env.Data, nil
}
