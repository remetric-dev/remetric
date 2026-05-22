// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"

	"github.com/remetric-dev/remetric/internal/config"
)

func newFlagSet() *pflag.FlagSet {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	config.BindFlags(fs)
	return fs
}

func TestLoad_Defaults(t *testing.T) {
	fs := newFlagSet()
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse: %v", err)
	}
	cfg, err := config.Load(fs, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Prometheus.MaxInFlight != 5 {
		t.Errorf("MaxInFlight = %d, want 5", cfg.Prometheus.MaxInFlight)
	}
	if cfg.Timeout.Seconds() != 300 {
		t.Errorf("Timeout = %v, want 5m", cfg.Timeout)
	}
}

func TestLoad_EnvOverridesDefaults(t *testing.T) {
	t.Setenv("REMETRIC_PROMETHEUS_URL", "http://from-env:9090")
	t.Setenv("REMETRIC_PROMETHEUS_MAX_IN_FLIGHT", "12")
	fs := newFlagSet()
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse: %v", err)
	}
	cfg, err := config.Load(fs, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Prometheus.URL != "http://from-env:9090" {
		t.Errorf("URL = %q, want from-env", cfg.Prometheus.URL)
	}
	if cfg.Prometheus.MaxInFlight != 12 {
		t.Errorf("MaxInFlight = %d, want 12", cfg.Prometheus.MaxInFlight)
	}
}

func TestLoad_FlagOverridesEnv(t *testing.T) {
	t.Setenv("REMETRIC_PROMETHEUS_URL", "http://from-env:9090")
	fs := newFlagSet()
	if err := fs.Parse([]string{"--prometheus", "http://from-flag:9090"}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	cfg, err := config.Load(fs, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Prometheus.URL != "http://from-flag:9090" {
		t.Errorf("URL = %q, want from-flag", cfg.Prometheus.URL)
	}
}

func TestLoad_FileLoading(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "remetric.yaml")
	body := []byte("prometheus:\n  url: http://from-file:9090\n  max_in_flight: 8\n")
	if err := os.WriteFile(cfgPath, body, 0o644); err != nil {
		t.Fatalf("write cfg: %v", err)
	}
	fs := newFlagSet()
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse: %v", err)
	}
	cfg, err := config.Load(fs, cfgPath)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Prometheus.URL != "http://from-file:9090" {
		t.Errorf("URL = %q, want from-file", cfg.Prometheus.URL)
	}
	if cfg.Prometheus.MaxInFlight != 8 {
		t.Errorf("MaxInFlight = %d, want 8", cfg.Prometheus.MaxInFlight)
	}
}

func TestLoad_NoColorEnv(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	fs := newFlagSet()
	_ = fs.Parse(nil)
	cfg, err := config.Load(fs, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.NoColor {
		t.Errorf("NoColor = false, want true (NO_COLOR set)")
	}
}

func TestConfig_NoProgressBinding(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	config.BindFlags(fs)
	if err := fs.Parse([]string{"--no-progress"}); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	cfg, err := config.Load(fs, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !cfg.NoProgress {
		t.Errorf("cfg.NoProgress = false, want true")
	}
}

func TestConfig_DefaultOutput(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	config.BindFlags(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	cfg, err := config.Load(fs, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Output != "terminal" {
		t.Errorf("Output = %q, want %q", cfg.Output, "terminal")
	}
}

func TestConfig_OutputFlagOverride(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	config.BindFlags(fs)
	if err := fs.Parse([]string{"--output", "json"}); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	cfg, err := config.Load(fs, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Output != "json" {
		t.Errorf("Output = %q, want %q", cfg.Output, "json")
	}
}

func TestConfig_OutputEnvOverride(t *testing.T) {
	t.Setenv("REMETRIC_OUTPUT", "json")
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	config.BindFlags(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	cfg, err := config.Load(fs, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Output != "json" {
		t.Errorf("Output = %q, want %q", cfg.Output, "json")
	}
}

func TestConfig_GrafanaFlags(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	config.BindFlags(fs)
	args := []string{
		"--grafana", "http://gra.example",
		"--grafana-token", "tok123",
		"--grafana-tls-skip-verify",
	}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	cfg, err := config.Load(fs, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Grafana.URL != "http://gra.example" {
		t.Errorf("URL = %q, want %q", cfg.Grafana.URL, "http://gra.example")
	}
	if cfg.Grafana.Token != "tok123" {
		t.Errorf("Token = %q, want %q", cfg.Grafana.Token, "tok123")
	}
	if !cfg.Grafana.TLSSkipVerify {
		t.Errorf("TLSSkipVerify = false, want true")
	}
}

func TestConfig_GrafanaEnvOverride(t *testing.T) {
	t.Setenv("REMETRIC_GRAFANA_URL", "http://env.example")
	t.Setenv("REMETRIC_GRAFANA_TOKEN", "envtok")
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	config.BindFlags(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	cfg, err := config.Load(fs, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Grafana.URL != "http://env.example" {
		t.Errorf("URL = %q, want %q", cfg.Grafana.URL, "http://env.example")
	}
	if cfg.Grafana.Token != "envtok" {
		t.Errorf("Token = %q, want %q", cfg.Grafana.Token, "envtok")
	}
}

func TestConfig_VMAlertFromEnv(t *testing.T) {
	t.Setenv("REMETRIC_VMALERT_URL", "http://vmalert:8880")
	t.Setenv("REMETRIC_VMALERT_TOKEN", "tok")
	t.Setenv("REMETRIC_BACKEND", "victoria")
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	config.BindFlags(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	cfg, err := config.Load(fs, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Backend != "victoria" {
		t.Errorf("Backend = %q, want %q", cfg.Backend, "victoria")
	}
	if cfg.VMAlert.URL != "http://vmalert:8880" {
		t.Errorf("VMAlert.URL = %q, want %q", cfg.VMAlert.URL, "http://vmalert:8880")
	}
	if cfg.VMAlert.Token != "tok" {
		t.Errorf("VMAlert.Token = %q, want %q", cfg.VMAlert.Token, "tok")
	}
}

func TestConfig_BackendDefault(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	config.BindFlags(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	cfg, err := config.Load(fs, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Backend != "auto" {
		t.Errorf("Backend default = %q, want %q", cfg.Backend, "auto")
	}
}

func TestConfig_VMAlertFlags(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	config.BindFlags(fs)
	args := []string{
		"--backend", "victoria",
		"--vmalert", "http://vmalert.example:8880",
		"--vmalert-token", "flagtok",
		"--vmalert-basic-auth", "user:pass",
		"--vmalert-tls-skip-verify",
	}
	if err := fs.Parse(args); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	cfg, err := config.Load(fs, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Backend != "victoria" {
		t.Errorf("Backend = %q, want %q", cfg.Backend, "victoria")
	}
	if cfg.VMAlert.URL != "http://vmalert.example:8880" {
		t.Errorf("VMAlert.URL = %q, want %q", cfg.VMAlert.URL, "http://vmalert.example:8880")
	}
	if cfg.VMAlert.Token != "flagtok" {
		t.Errorf("VMAlert.Token = %q, want %q", cfg.VMAlert.Token, "flagtok")
	}
	if cfg.VMAlert.BasicAuth != "user:pass" {
		t.Errorf("VMAlert.BasicAuth = %q, want %q", cfg.VMAlert.BasicAuth, "user:pass")
	}
	if !cfg.VMAlert.TLSSkipVerify {
		t.Errorf("VMAlert.TLSSkipVerify = false, want true")
	}
}

func TestConfig_Validate_PromBothAuthRejected(t *testing.T) {
	cfg := &config.Config{
		Prometheus: config.PrometheusConfig{Token: "tok", BasicAuth: "user:pass"},
		Backend:    "auto",
	}
	if err := cfg.Validate(); err == nil {
		t.Errorf("expected error for both prom auth flags set")
	}
}

func TestConfig_Validate_VMAlertBothAuthRejected(t *testing.T) {
	cfg := &config.Config{
		VMAlert: config.VMAlertConfig{Token: "tok", BasicAuth: "user:pass"},
		Backend: "auto",
	}
	if err := cfg.Validate(); err == nil {
		t.Errorf("expected error for both vmalert auth flags set")
	}
}

func TestConfig_Validate_UnknownBackendRejected(t *testing.T) {
	cfg := &config.Config{Backend: "mimir"}
	if err := cfg.Validate(); err == nil {
		t.Errorf("expected error for unknown backend %q", cfg.Backend)
	}
}

func TestConfig_Validate_AcceptsValidConfig(t *testing.T) {
	cases := []*config.Config{
		{Backend: "auto"},
		{Backend: "prometheus"},
		{Backend: "victoria"},
		{Backend: ""}, // empty == auto, accepted
		{Backend: "auto", Prometheus: config.PrometheusConfig{Token: "t"}},
		{Backend: "auto", Prometheus: config.PrometheusConfig{BasicAuth: "u:p"}},
		{Backend: "auto", VMAlert: config.VMAlertConfig{URL: "http://vmalert:8880", Token: "t"}},
	}
	for i, cfg := range cases {
		if err := cfg.Validate(); err != nil {
			t.Errorf("case %d: Validate() = %v, want nil", i, err)
		}
	}
}
