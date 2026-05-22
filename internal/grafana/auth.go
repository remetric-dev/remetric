// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package grafana

import "net/http"

// auth is implemented by every authentication strategy; apply
// installs credentials on the outgoing request.
type auth interface {
	apply(*http.Request)
}

// noAuth makes anonymous requests. Used for Grafana instances with
// anonymous access (e.g. our e2e fixture).
type noAuth struct{}

func (noAuth) apply(*http.Request) {}

// bearerAuth installs `Authorization: Bearer <token>` - used by
// Grafana service-account API keys.
type bearerAuth struct{ token string }

func (b bearerAuth) apply(r *http.Request) {
	r.Header.Set("Authorization", "Bearer "+b.token)
}

// basicAuth installs HTTP basic auth.
type basicAuth struct {
	user, pass string
}

func (a basicAuth) apply(r *http.Request) {
	r.SetBasicAuth(a.user, a.pass)
}
