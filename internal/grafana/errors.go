// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

// Package grafana is a minimal HTTP client for the Grafana REST API.
// It supports listing dashboards and fetching dashboard JSON for the
// purpose of extracting PromQL queries.
package grafana

import "errors"

// Sentinel errors returned by Client constructors and methods.
var (
	ErrInvalidURL      = errors.New("invalid url")
	ErrAuth            = errors.New("authentication failed")
	ErrNotFound        = errors.New("resource not found")
	ErrInvalidArgument = errors.New("invalid argument")
	ErrConflictingAuth = errors.New("conflicting auth options")
)
