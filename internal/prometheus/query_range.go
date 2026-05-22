// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// QueryRangeResult is the parsed /api/v1/query_range response.
type QueryRangeResult struct {
	ResultType string   `json:"resultType"`
	Result     []Series `json:"result"`
}

// Series is one matrix series - label set plus an ordered list of samples.
type Series struct {
	Metric map[string]string `json:"metric"`
	Values []SamplePair      `json:"values"`
}

// SamplePair is a single (timestamp, value) point. JSON form is a two-element
// array [unix_seconds_float, "value_string"]. Timestamps decode at millisecond
// precision to match Prometheus's underlying model.Time representation.
type SamplePair struct {
	Timestamp time.Time
	Value     float64
}

// UnmarshalJSON parses Prometheus's [ts, "val"] tuple shape.
func (s *SamplePair) UnmarshalJSON(data []byte) error {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("sample pair: %w", err)
	}
	if len(raw) != 2 {
		return fmt.Errorf("sample pair: expected 2 elements, got %d", len(raw))
	}
	var tsFloat float64
	if err := json.Unmarshal(raw[0], &tsFloat); err != nil {
		return fmt.Errorf("sample pair timestamp: %w", err)
	}
	// Prometheus emits timestamps as float seconds but the underlying precision is
	// milliseconds (model.Time is int64 ms). Round to ms to avoid float drift, then
	// reconstruct as time.Time at ms precision in UTC.
	ms := int64(math.Round(tsFloat * 1000))
	s.Timestamp = time.UnixMilli(ms).UTC()

	var valStr string
	if err := json.Unmarshal(raw[1], &valStr); err != nil {
		return fmt.Errorf("sample pair value (must be string): %w", err)
	}
	v, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return fmt.Errorf("sample pair value parse: %w", err)
	}
	s.Value = v
	return nil
}

// QueryRange calls /api/v1/query_range. step must be > 0.
// start and end are inclusive; they're sent as Unix seconds (no fractional).
func (c *Client) QueryRange(ctx context.Context, expr string, start, end time.Time, step time.Duration) (*QueryRangeResult, error) {
	if step <= 0 {
		return nil, fmt.Errorf("remetric: %w: step must be > 0", ErrInvalidArgument)
	}
	q := url.Values{}
	q.Set("query", expr)
	q.Set("start", strconv.FormatInt(start.Unix(), 10))
	q.Set("end", strconv.FormatInt(end.Unix(), 10))
	q.Set("step", strconv.FormatInt(int64(step.Seconds()), 10))
	body, err := c.do(ctx, http.MethodGet, "/api/v1/query_range?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	var env envelope[QueryRangeResult]
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("remetric: parse query_range: %w", err)
	}
	return &env.Data, nil
}
