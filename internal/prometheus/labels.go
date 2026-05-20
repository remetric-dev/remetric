// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

// LabelNamesForMetric returns the list of label names attached to the given
// metric. The "__name__" pseudo-label is filtered out.
func (c *Client) LabelNamesForMetric(ctx context.Context, metric string) ([]string, error) {
	q := url.Values{}
	q.Add("match[]", fmt.Sprintf(`{__name__="%s"}`, metric))
	body, err := c.do(ctx, http.MethodGet, "/api/v1/labels?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var env envelope[[]string]
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("remetric: parse labels: %w", err)
	}
	out := make([]string, 0, len(env.Data))
	for _, name := range env.Data {
		if name == "__name__" {
			continue
		}
		out = append(out, name)
	}
	return out, nil
}

// MetricNamesWithLabel returns the set of metric names with at least
// one series that carries label. Implemented via:
//
//	GET /api/v1/label/__name__/values?match[]={<label>!=""}
//
// An empty label returns ErrInvalidArgument.
func (c *Client) MetricNamesWithLabel(ctx context.Context, label string) ([]string, error) {
	if label == "" {
		return nil, fmt.Errorf("remetric: %w: empty label", ErrInvalidArgument)
	}
	q := url.Values{}
	q.Add("match[]", fmt.Sprintf(`{%s!=""}`, label))
	body, err := c.do(ctx, http.MethodGet, "/api/v1/label/__name__/values?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var env envelope[[]string]
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("remetric: parse metric names: %w", err)
	}
	return env.Data, nil
}

// LabelValues returns the values for label, optionally restricted by one or
// more PromQL selectors (e.g. `{__name__="istio_requests_total"}`).
func (c *Client) LabelValues(ctx context.Context, label string, matchers ...string) ([]string, error) {
	path := "/api/v1/label/" + url.PathEscape(label) + "/values"
	if len(matchers) > 0 {
		q := url.Values{}
		for _, m := range matchers {
			q.Add("match[]", m)
		}
		path += "?" + q.Encode()
	}
	body, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var env envelope[[]string]
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("remetric: parse label values: %w", err)
	}
	return env.Data, nil
}
