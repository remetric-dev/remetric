// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

// Package prometheus is a small HTTP client for the Prometheus HTTP API.
package prometheus

import (
	"errors"
	"fmt"
)

// Sentinel errors usable with errors.Is.
var (
	ErrAuth              = errors.New("prometheus authentication failed")
	ErrNotFound          = errors.New("prometheus endpoint not found")
	ErrPrometheusVersion = errors.New("prometheus version too old")
	ErrConflictingAuth   = errors.New("conflicting auth options")
	ErrInvalidURL        = errors.New("invalid URL")
)

// Error captures the HTTP details of a failed Prometheus call.
type Error struct {
	StatusCode int
	Status     string
	Method     string
	URL        string
	Body       string

	// sentinel is the categorical error to expose via errors.Is.
	sentinel error
}

// Error formats the error, truncating the body to 1024 chars.
func (e *Error) Error() string {
	body := e.Body
	const maxBody = 1024
	if len(body) > maxBody {
		body = body[:maxBody] + "...[truncated]"
	}
	return fmt.Sprintf("prometheus: %s %s -> %s: %s", e.Method, e.URL, e.Status, body)
}

// Unwrap exposes the sentinel for errors.Is.
func (e *Error) Unwrap() error { return e.sentinel }
