// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package grafana

import (
	"reflect"
	"testing"
	"time"
)

func TestOptions_Apply(t *testing.T) {
	c := &Client{}
	WithBearerToken("t")(c)
	if _, ok := c.auth.(bearerAuth); !ok {
		t.Errorf("auth type = %T, want bearerAuth", c.auth)
	}

	WithBasicAuth("u", "p")(c)
	if _, ok := c.auth.(basicAuth); !ok {
		t.Errorf("auth type = %T, want basicAuth", c.auth)
	}

	WithTLSSkipVerify(true)(c)
	if !c.tlsSkipVerify {
		t.Error("tlsSkipVerify = false, want true")
	}

	WithTimeout(7 * time.Second)(c)
	if c.timeout != 7*time.Second {
		t.Errorf("timeout = %v, want 7s", c.timeout)
	}

	WithMaxInFlight(3)(c)
	if c.maxInFlight != 3 {
		t.Errorf("maxInFlight = %d, want 3", c.maxInFlight)
	}

	WithUserAgent("ua")(c)
	if c.userAgent != "ua" {
		t.Errorf("userAgent = %q, want %q", c.userAgent, "ua")
	}
}

// Defensive: ensure auth types compare correctly for ErrConflictingAuth detection.
func TestAuthTypes_Distinguishable(t *testing.T) {
	a := any(bearerAuth{token: "x"})
	b := any(basicAuth{user: "u", pass: "p"})
	if reflect.TypeOf(a) == reflect.TypeOf(b) {
		t.Error("bearerAuth and basicAuth share a type")
	}
}
