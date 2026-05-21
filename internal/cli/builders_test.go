// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package cli

import (
	"testing"

	"github.com/remetric-dev/remetric/internal/config"
	prom "github.com/remetric-dev/remetric/internal/prometheus"
)

func TestBuildVMAlertClient_NilWhenURLEmpty(t *testing.T) {
	cfg := &config.Config{}
	c, err := buildVMAlertClient(cfg, "test-agent")
	if err != nil {
		t.Fatalf("buildVMAlertClient: %v", err)
	}
	if c != nil {
		t.Errorf("expected nil client when URL empty, got %v", c)
	}
}

func TestBuildVMAlertClient_RejectsBothAuthFlags(t *testing.T) {
	cfg := &config.Config{
		VMAlert: config.VMAlertConfig{
			URL:       "http://vmalert:8880",
			Token:     "tok",
			BasicAuth: "user:pass",
		},
	}
	if _, err := buildVMAlertClient(cfg, "test-agent"); err == nil {
		t.Errorf("expected error for conflicting auth flags")
	}
}

func TestBuildVMAlertClient_InheritsPromAuth(t *testing.T) {
	cfg := &config.Config{
		Prometheus: config.PrometheusConfig{Token: "shared-token"},
		VMAlert:    config.VMAlertConfig{URL: "http://vmalert:8880"},
	}
	c, err := buildVMAlertClient(cfg, "test-agent")
	if err != nil {
		t.Fatalf("buildVMAlertClient: %v", err)
	}
	if c == nil {
		t.Fatalf("expected non-nil client when URL set")
	}
}

func TestBuildVMAlertClient_OwnTokenWinsOverInherited(t *testing.T) {
	cfg := &config.Config{
		Prometheus: config.PrometheusConfig{Token: "prom-token"},
		VMAlert: config.VMAlertConfig{
			URL:   "http://vmalert:8880",
			Token: "vmalert-token",
		},
	}
	c, err := buildVMAlertClient(cfg, "test-agent")
	if err != nil {
		t.Fatalf("buildVMAlertClient: %v", err)
	}
	if c == nil {
		t.Fatalf("expected non-nil client")
	}
}

func TestBuildVMAlertClient_InvalidBasicAuthRejected(t *testing.T) {
	cfg := &config.Config{
		VMAlert: config.VMAlertConfig{
			URL:       "http://vmalert:8880",
			BasicAuth: "no-colon-here",
		},
	}
	if _, err := buildVMAlertClient(cfg, "test-agent"); err == nil {
		t.Errorf("expected error for malformed basic auth")
	}
}

func TestBuildPromClient_BackendHintApplied(t *testing.T) {
	cases := []struct {
		backend string
		want    prom.Flavor
	}{
		{"", prom.FlavorUnknown},
		{"auto", prom.FlavorUnknown},
		{"prometheus", prom.FlavorProm},
		{"victoria", prom.FlavorVictoria},
	}
	for _, tc := range cases {
		cfg := &config.Config{
			Prometheus: config.PrometheusConfig{URL: "http://localhost:9090", MaxInFlight: 1},
			Backend:    tc.backend,
		}
		c, err := buildPromClient(cfg, "test-agent")
		if err != nil {
			t.Fatalf("backend=%q: %v", tc.backend, err)
		}
		if got := c.FlavorHint(); got != tc.want {
			t.Errorf("backend=%q: FlavorHint() = %v, want %v", tc.backend, got, tc.want)
		}
	}
}

func TestSplitBasicAuth(t *testing.T) {
	cases := []struct {
		in       string
		wantUser string
		wantPass string
		wantOk   bool
	}{
		{"user:pass", "user", "pass", true},
		{"user:", "user", "", true},
		{":pass", "", "pass", true},
		{"user:pass:with:colons", "user", "pass:with:colons", true},
		{"no-colon", "", "", false},
		{"", "", "", false},
	}
	for _, tc := range cases {
		u, p, ok := splitBasicAuth(tc.in)
		if u != tc.wantUser || p != tc.wantPass || ok != tc.wantOk {
			t.Errorf("splitBasicAuth(%q) = (%q, %q, %v), want (%q, %q, %v)", tc.in, u, p, ok, tc.wantUser, tc.wantPass, tc.wantOk)
		}
	}
}
