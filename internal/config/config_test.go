// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/pflag"

	"github.com/remetric-dev/remetric/internal/config"
	"github.com/remetric-dev/remetric/internal/findings"
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

func TestConfig_FailOnBinding_DefaultIsNone(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	config.BindFlags(fs)
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	cfg, err := config.Load(fs, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	_, enabled := cfg.FailOnThreshold()
	if enabled {
		t.Errorf("FailOnThreshold enabled by default, want disabled")
	}
}

func TestConfig_FailOn_ParsesValidValue(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	config.BindFlags(fs)
	if err := fs.Parse([]string{"--fail-on", "critical"}); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	cfg, err := config.Load(fs, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	sev, enabled := cfg.FailOnThreshold()
	if !enabled {
		t.Errorf("enabled = false, want true for --fail-on=critical")
	}
	if sev != findings.SeverityCritical {
		t.Errorf("sev = %v, want SeverityCritical", sev)
	}
}

func TestConfig_FailOn_RejectsInvalidValue(t *testing.T) {
	fs := pflag.NewFlagSet("test", pflag.ContinueOnError)
	config.BindFlags(fs)
	if err := fs.Parse([]string{"--fail-on", "nope"}); err != nil {
		t.Fatalf("Parse: %v", err)
	}
	cfg, err := config.Load(fs, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Validate(); err == nil {
		t.Errorf("Validate accepted --fail-on=nope, want error")
	}
}

func TestLoad_IgnoreFlags(t *testing.T) {
	fs := newFlagSet()
	if err := fs.Parse([]string{
		"--ignore-metric", "node_.*",
		"--ignore-metric", "go_.*",
		"--ignore-label", "pod",
		"--ignore-alert", "HighMemoryUsage",
	}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	cfg, err := config.Load(fs, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got := len(cfg.Ignore.Metric); got != 2 {
		t.Errorf("Ignore.Metric = %v, want 2 entries", cfg.Ignore.Metric)
	}
	if got := len(cfg.Ignore.Label); got != 1 {
		t.Errorf("Ignore.Label = %v, want 1 entry", cfg.Ignore.Label)
	}
	if got := len(cfg.Ignore.Alert); got != 1 {
		t.Errorf("Ignore.Alert = %v, want 1 entry", cfg.Ignore.Alert)
	}
	if cfg.IgnoreFilter() == nil {
		t.Error("IgnoreFilter() = nil, want non-nil after Validate")
	}
}

func TestLoad_IgnoreEnv(t *testing.T) {
	t.Setenv("REMETRIC_IGNORE_METRIC", "node_.*,go_.*")
	fs := newFlagSet()
	if err := fs.Parse(nil); err != nil {
		t.Fatalf("parse: %v", err)
	}
	cfg, err := config.Load(fs, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got := len(cfg.Ignore.Metric); got != 2 {
		t.Errorf("Ignore.Metric (env) = %v, want 2 entries", cfg.Ignore.Metric)
	}
}

func TestLoad_IgnoreYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "remetric.yaml")
	body := []byte("ignore:\n  metric:\n    - node_.*\n    - go_.*\n  label:\n    - pod\n  alert:\n    - HighMemoryUsage\n")
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
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	if got := len(cfg.Ignore.Metric); got != 2 {
		t.Errorf("Ignore.Metric (yaml) = %v, want 2 entries", cfg.Ignore.Metric)
	}
	if got := len(cfg.Ignore.Label); got != 1 {
		t.Errorf("Ignore.Label (yaml) = %v, want 1 entry", cfg.Ignore.Label)
	}
	if got := len(cfg.Ignore.Alert); got != 1 {
		t.Errorf("Ignore.Alert (yaml) = %v, want 1 entry", cfg.Ignore.Alert)
	}
}

func TestLoad_IgnoreBadRegexFailsValidate(t *testing.T) {
	fs := newFlagSet()
	if err := fs.Parse([]string{"--ignore-metric", "foo["}); err != nil {
		t.Fatalf("parse: %v", err)
	}
	cfg, err := config.Load(fs, "")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate returned nil; want error on bad regex")
	}
}
