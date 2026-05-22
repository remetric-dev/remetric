// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package prometheus

import (
	"log/slog"
	"testing"
	"time"
)

func TestOptions_Apply(t *testing.T) {
	c := &Client{}
	WithBearerToken("tok")(c)
	if _, ok := c.auth.(bearerAuth); !ok {
		t.Errorf("auth = %T, want bearerAuth", c.auth)
	}

	c = &Client{}
	WithBasicAuth("u", "p")(c)
	if _, ok := c.auth.(basicAuth); !ok {
		t.Errorf("auth = %T, want basicAuth", c.auth)
	}

	c = &Client{}
	WithTLSSkipVerify(true)(c)
	if !c.tlsSkipVerify {
		t.Errorf("tlsSkipVerify = false, want true")
	}

	c = &Client{}
	WithTimeout(7 * time.Second)(c)
	if c.timeout != 7*time.Second {
		t.Errorf("timeout = %v, want 7s", c.timeout)
	}

	c = &Client{}
	WithMaxInFlight(12)(c)
	if c.maxInFlight != 12 {
		t.Errorf("maxInFlight = %d, want 12", c.maxInFlight)
	}

	c = &Client{}
	WithUserAgent("ua/1.0")(c)
	if c.userAgent != "ua/1.0" {
		t.Errorf("userAgent = %q, want ua/1.0", c.userAgent)
	}

	c = &Client{}
	l := slog.Default()
	WithLogger(l)(c)
	if c.logger != l {
		t.Errorf("logger not stored")
	}
}

func TestWithMaxInFlight_GetterExposesValue(t *testing.T) {
	c, err := New("http://example.invalid", WithMaxInFlight(12))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := c.MaxInFlight(); got != 12 {
		t.Errorf("c.MaxInFlight() = %d, want 12", got)
	}
}

func TestMaxInFlight_DefaultsToFive(t *testing.T) {
	c, err := New("http://example.invalid")
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	if got := c.MaxInFlight(); got != 5 {
		t.Errorf("c.MaxInFlight() = %d, want 5", got)
	}
}
