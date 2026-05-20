// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package grafana

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func TestSearch_SinglePage(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/search" {
			t.Fatalf("unexpected path %q", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"uid": "a", "title": "Alpha", "url": "/d/a/alpha"},
			{"uid": "b", "title": "Beta", "url": "/d/b/beta"},
		})
	}))
	defer ts.Close()

	c, _ := New(ts.URL)
	got, err := c.Search(context.Background())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	want := []DashboardRef{
		{UID: "a", Title: "Alpha", URL: "/d/a/alpha"},
		{UID: "b", Title: "Beta", URL: "/d/b/beta"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Search mismatch (-want +got):\n%s", diff)
	}
}

func TestSearch_Paginated(t *testing.T) {
	// Page 1 returns 1000 dashboards; page 2 returns 3; page 3 empty.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		page := r.URL.Query().Get("page")
		w.Header().Set("Content-Type", "application/json")
		switch page {
		case "1":
			out := make([]map[string]any, 1000)
			for i := range out {
				out[i] = map[string]any{
					"uid":   fmt.Sprintf("uid-%d", i),
					"title": fmt.Sprintf("Title %d", i),
				}
			}
			_ = json.NewEncoder(w).Encode(out)
		case "2":
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"uid": "p2-0", "title": "Late 0"},
				{"uid": "p2-1", "title": "Late 1"},
				{"uid": "p2-2", "title": "Late 2"},
			})
		default:
			_ = json.NewEncoder(w).Encode([]map[string]any{})
		}
	}))
	defer ts.Close()

	c, _ := New(ts.URL)
	got, err := c.Search(context.Background())
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1003 {
		t.Errorf("len = %d, want 1003", len(got))
	}
	if got[0].UID != "uid-0" || got[1002].UID != "p2-2" {
		t.Errorf("pagination order broken: first=%q, last=%q", got[0].UID, got[1002].UID)
	}
}

func TestSearch_PassesType(t *testing.T) {
	var sawType string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawType = r.URL.Query().Get("type")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[]`))
	}))
	defer ts.Close()

	c, _ := New(ts.URL)
	_, _ = c.Search(context.Background())
	if sawType != "dash-db" {
		t.Errorf("type query = %q, want dash-db", sawType)
	}
}
