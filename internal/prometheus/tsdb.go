// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package prometheus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// BuildInfo summarises /api/v1/status/buildinfo.
type BuildInfo struct {
	Version   string `json:"version"`
	Revision  string `json:"revision"`
	GoVersion string `json:"goVersion"`
	BuildUser string `json:"buildUser"`
	BuildDate string `json:"buildDate"`
}

// RuntimeInfo summarises /api/v1/status/runtimeinfo.
type RuntimeInfo struct {
	StartTime        string `json:"startTime"`
	LastConfigTime   string `json:"lastConfigTime"`
	GoroutineCount   int    `json:"goroutineCount"`
	StorageRetention string `json:"storageRetention"`
}

// NameValue is a key/count pair from TSDB stats.
type NameValue struct {
	Name  string `json:"name"`
	Value int64  `json:"value"`
}

// HeadStats describes the in-memory head block of the TSDB.
type HeadStats struct {
	NumSeries     int64 `json:"numSeries"`
	NumLabelPairs int64 `json:"numLabelPairs"`
	ChunkCount    int64 `json:"chunkCount"`
	MinTime       int64 `json:"minTime"`
	MaxTime       int64 `json:"maxTime"`
}

// TSDBStats is the parsed /api/v1/status/tsdb response.
type TSDBStats struct {
	HeadStats                   HeadStats   `json:"headStats"`
	SeriesCountByMetricName     []NameValue `json:"seriesCountByMetricName"`
	LabelValueCountByLabelName  []NameValue `json:"labelValueCountByLabelName"`
	MemoryInBytesByLabelName    []NameValue `json:"memoryInBytesByLabelName"`
	SeriesCountByLabelValuePair []NameValue `json:"seriesCountByLabelValuePair"`
}

// envelope wraps Prometheus's "status" response shape.
type envelope[T any] struct {
	Status string `json:"status"`
	Data   T      `json:"data"`
}

// Ping checks /-/healthy.
func (c *Client) Ping(ctx context.Context) error {
	_, err := c.do(ctx, http.MethodGet, "/-/healthy", nil)
	return err
}

// BuildInfo fetches and parses /api/v1/status/buildinfo.
// When flavor detection has already populated the cache, the cached result
// is returned without issuing a second HTTP request.
func (c *Client) BuildInfo(ctx context.Context) (*BuildInfo, error) {
	if err := c.ensureFlavor(ctx); err != nil {
		return nil, err
	}
	if c.buildInfoCache != nil {
		return c.buildInfoCache, nil
	}
	// Fallback path: cache wasn't populated (e.g., flavor hint skipped detection).
	body, err := c.do(ctx, http.MethodGet, "/api/v1/status/buildinfo", nil)
	if err != nil {
		return nil, err
	}
	var env envelope[BuildInfo]
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("remetric: parse buildinfo: %w", err)
	}
	c.buildInfoCache = &env.Data
	return c.buildInfoCache, nil
}

// RuntimeInfo fetches and parses /api/v1/status/runtimeinfo.
func (c *Client) RuntimeInfo(ctx context.Context) (*RuntimeInfo, error) {
	body, err := c.do(ctx, http.MethodGet, "/api/v1/status/runtimeinfo", nil)
	if err != nil {
		return nil, err
	}
	var env envelope[RuntimeInfo]
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("remetric: parse runtimeinfo: %w", err)
	}
	return &env.Data, nil
}

// TSDBStats fetches and parses /api/v1/status/tsdb.
// limit caps the top-N lists; pass 0 for server default.
func (c *Client) TSDBStats(ctx context.Context, limit int) (*TSDBStats, error) {
	path := "/api/v1/status/tsdb"
	if limit > 0 {
		path += fmt.Sprintf("?limit=%d", limit)
	}
	body, err := c.do(ctx, http.MethodGet, path, nil)
	if err != nil {
		return nil, err
	}
	var env envelope[TSDBStats]
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, fmt.Errorf("remetric: parse tsdb stats: %w", err)
	}
	return &env.Data, nil
}
