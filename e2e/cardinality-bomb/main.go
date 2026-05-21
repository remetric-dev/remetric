// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

// Command cardinality-bomb emits Prometheus text-format metrics with
// deliberately unbounded labels (user_id, trace_id, request_id, path)
// so that remetric's analyzers have something interesting to flag in
// the e2e stack.
//
// This is e2e-only test scaffolding; it is not part of the remetric
// product and is excluded from the main go.mod.
package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"strings"
)

func main() {
	body := buildMetrics(500)

	http.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		_, _ = fmt.Fprint(w, body)
	})
	http.HandleFunc("/", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintln(w, "cardinality-bomb — see /metrics")
	})

	addr := ":8080"
	if v := os.Getenv("LISTEN"); v != "" {
		addr = v
	}
	fmt.Println("cardinality-bomb listening on", addr)
	if err := http.ListenAndServe(addr, nil); err != nil { //nolint:gosec // dev-only e2e service
		fmt.Fprintln(os.Stderr, "listen:", err)
		os.Exit(1)
	}
}

// buildMetrics renders a single static block of Prometheus
// text-format metrics. n distinct (user_id, trace_id) combinations
// are emitted under app_requests_total with random paths and counts;
// a smaller orphan_metric is also emitted so the unused analyzer has
// an obvious target.
func buildMetrics(n int) string {
	paths := []string{"/api/users", "/api/orders", "/api/items", "/health", "/login"}

	var sb strings.Builder
	sb.WriteString("# HELP app_requests_total Total app requests by user_id, trace_id, path.\n")
	sb.WriteString("# TYPE app_requests_total counter\n")
	for i := 0; i < n; i++ {
		uid := fmt.Sprintf("usr-%s-%d", randHex(8), i)
		tid := fmt.Sprintf("trace-%s", randHex(16))
		path := paths[rand.Intn(len(paths))]
		fmt.Fprintf(&sb, `app_requests_total{user_id="%s",trace_id="%s",path="%s"} %d`+"\n",
			uid, tid, path, rand.Intn(100))
	}

	// A metric nobody references — useful for `remetric metrics unused`.
	sb.WriteString("\n# HELP orphan_metric_total A metric exposed but never queried.\n")
	sb.WriteString("# TYPE orphan_metric_total counter\n")
	sb.WriteString("orphan_metric_total 42\n")

	return sb.String()
}

func randHex(n int) string {
	const chars = "0123456789abcdef"
	out := make([]byte, n)
	for i := range out {
		out[i] = chars[rand.Intn(len(chars))]
	}
	return string(out)
}
