// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
)

// DashboardRef is the minimal pointer to a dashboard returned by
// /api/search. UID is enough to fetch the full body via Client.Dashboard.
type DashboardRef struct {
	UID   string `json:"uid"`
	Title string `json:"title"`
	URL   string `json:"url,omitempty"`
}

// searchPageSize matches Grafana's documented maximum page size.
const searchPageSize = 1000

// Search lists every dashboard (type=dash-db) across the Grafana
// instance, paginating until an empty page is returned.
func (c *Client) Search(ctx context.Context) ([]DashboardRef, error) {
	var all []DashboardRef
	for page := 1; ; page++ {
		q := url.Values{}
		q.Set("type", "dash-db")
		q.Set("limit", strconv.Itoa(searchPageSize))
		q.Set("page", strconv.Itoa(page))
		body, err := c.do(ctx, http.MethodGet, "/api/search?"+q.Encode(), nil)
		if err != nil {
			return nil, fmt.Errorf("grafana: search page %d: %w", page, err)
		}
		var refs []DashboardRef
		if err := json.Unmarshal(body, &refs); err != nil {
			return nil, fmt.Errorf("grafana: parse search page %d: %w", page, err)
		}
		if len(refs) == 0 {
			break
		}
		all = append(all, refs...)
		if len(refs) < searchPageSize {
			break
		}
	}
	return all, nil
}
