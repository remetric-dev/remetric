// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

// Package config holds the layered runtime configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/remetric-dev/remetric/internal/findings"
	"github.com/remetric-dev/remetric/internal/ignore"
)

// Config is the fully-resolved runtime configuration.
type Config struct {
	Prometheus PrometheusConfig
	Grafana    GrafanaConfig
	VMAlert    VMAlertConfig
	Backend    string `mapstructure:"backend"`
	Verbose    bool
	Timeout    time.Duration
	NoColor    bool         `mapstructure:"no_color"`
	NoProgress bool         `mapstructure:"no_progress"`
	Output     string       `mapstructure:"output"`
	FailOn     string       `mapstructure:"fail_on"`
	Ignore     IgnoreConfig `mapstructure:"ignore"`

	failOnSeverity findings.Severity
	failOnEnabled  bool
	ignoreFilter   *ignore.Filter
}

// IgnoreConfig holds the raw regex patterns to drop findings whose
// structured fields match. Parsed once in Validate.
type IgnoreConfig struct {
	Metric []string `mapstructure:"metric"`
	Label  []string `mapstructure:"label"`
	Alert  []string `mapstructure:"alert"`
}

// PrometheusConfig holds Prometheus client options.
type PrometheusConfig struct {
	URL           string `mapstructure:"url"`
	Token         string `mapstructure:"token"`
	BasicAuth     string `mapstructure:"basic_auth"`
	TLSSkipVerify bool   `mapstructure:"tls_skip_verify"`
	MaxInFlight   int    `mapstructure:"max_in_flight"`
}

// GrafanaConfig holds Grafana client options.
type GrafanaConfig struct {
	URL           string `mapstructure:"url"`
	Token         string `mapstructure:"token"`
	BasicAuth     string `mapstructure:"basic_auth"`
	TLSSkipVerify bool   `mapstructure:"tls_skip_verify"`
	MaxInFlight   int    `mapstructure:"max_in_flight"`
}

// VMAlertConfig holds vmalert client options. If URL is empty, callers
// fall back to the Prometheus client for /api/v1/rules.
type VMAlertConfig struct {
	URL           string `mapstructure:"url"`
	Token         string `mapstructure:"token"`
	BasicAuth     string `mapstructure:"basic_auth"`
	TLSSkipVerify bool   `mapstructure:"tls_skip_verify"`
}

// BindFlags wires pflag flags into a flag set. The same flag set is later
// passed to Load.
func BindFlags(fs *pflag.FlagSet) {
	fs.String("prometheus", "", "Prometheus base URL")
	fs.String("prom-token", "", "Bearer token for Prometheus")
	fs.String("prom-basic-auth", "", "Basic auth as user:password")
	fs.Bool("prom-tls-skip-verify", false, "Skip TLS verification")
	fs.Int("prom-max-in-flight", 5, "Max concurrent Prometheus requests")
	fs.String("grafana", "", "Grafana base URL")
	fs.String("grafana-token", "", "Bearer token (service-account API key) for Grafana")
	fs.String("grafana-basic-auth", "", "Basic auth as user:password for Grafana")
	fs.Bool("grafana-tls-skip-verify", false, "Skip TLS verification for Grafana")
	fs.Int("grafana-max-in-flight", 5, "Max concurrent Grafana requests")
	fs.String("backend", "auto", "Backend dialect: auto|prometheus|victoria")
	fs.String("vmalert", "", "vmalert base URL for /api/v1/rules")
	fs.String("vmalert-token", "", "Bearer token for vmalert (defaults to --prom-token)")
	fs.String("vmalert-basic-auth", "", "Basic auth as user:password for vmalert (defaults to --prom-basic-auth)")
	fs.Bool("vmalert-tls-skip-verify", false, "Skip TLS verification for vmalert")
	fs.Bool("no-color", false, "Disable colored output")
	fs.Bool("no-progress", false, "Disable per-phase progress on stderr (auto-off when not a TTY)")
	fs.String("output", "terminal", "Output format: terminal|json")
	fs.String("fail-on", "none", "Exit non-zero (3) when any finding is at or above this severity: low|medium|high|critical|none")
	fs.StringArray("ignore-metric", nil, "Drop findings whose metric name matches this regex (repeatable). Anchored full-match.")
	fs.StringArray("ignore-label", nil, "Drop findings whose evidence label matches this regex (repeatable). Anchored full-match.")
	fs.StringArray("ignore-alert", nil, "Drop findings whose alert name matches this regex (repeatable). Anchored full-match.")
	fs.BoolP("verbose", "v", false, "Verbose logging")
	fs.Duration("timeout", 5*time.Minute, "Total operation timeout")
	fs.String("config", "", "Path to config file")
}

// Load merges defaults, file, env, and flags (last wins) into a Config.
func Load(fs *pflag.FlagSet, cfgPath string) (*Config, error) {
	v := viper.New()
	v.SetEnvPrefix("REMETRIC")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	v.AutomaticEnv()

	v.SetDefault("prometheus.max_in_flight", 5)
	v.SetDefault("grafana.max_in_flight", 5)
	v.SetDefault("timeout", 5*time.Minute)
	v.SetDefault("output", "terminal")
	v.SetDefault("backend", "auto")

	bindings := map[string]string{
		"prometheus":              "prometheus.url",
		"prom-token":              "prometheus.token",
		"prom-basic-auth":         "prometheus.basic_auth",
		"prom-tls-skip-verify":    "prometheus.tls_skip_verify",
		"prom-max-in-flight":      "prometheus.max_in_flight",
		"grafana":                 "grafana.url",
		"grafana-token":           "grafana.token",
		"grafana-basic-auth":      "grafana.basic_auth",
		"grafana-tls-skip-verify": "grafana.tls_skip_verify",
		"grafana-max-in-flight":   "grafana.max_in_flight",
		"backend":                 "backend",
		"vmalert":                 "vmalert.url",
		"vmalert-token":           "vmalert.token",
		"vmalert-basic-auth":      "vmalert.basic_auth",
		"vmalert-tls-skip-verify": "vmalert.tls_skip_verify",
		"no-color":                "no_color",
		"no-progress":             "no_progress",
		"output":                  "output",
		"fail-on":                 "fail_on",
		"ignore-metric":           "ignore.metric",
		"ignore-label":            "ignore.label",
		"ignore-alert":            "ignore.alert",
		"verbose":                 "verbose",
		"timeout":                 "timeout",
	}
	for flagName, key := range bindings {
		if f := fs.Lookup(flagName); f != nil {
			_ = v.BindPFlag(key, f)
		}
	}

	if cfgPath != "" {
		v.SetConfigFile(cfgPath)
	} else {
		v.SetConfigName(".remetric")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		if home, err := os.UserHomeDir(); err == nil {
			v.AddConfigPath(home + "/.config/remetric")
		}
	}
	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, fmt.Errorf("read config: %w", err)
		}
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	if os.Getenv("NO_COLOR") != "" {
		cfg.NoColor = true
	}

	return &cfg, nil
}

// FailOnThreshold returns the parsed --fail-on severity and whether the
// gate is enabled. Result is valid only after Validate has succeeded.
func (c *Config) FailOnThreshold() (findings.Severity, bool) {
	return c.failOnSeverity, c.failOnEnabled
}

// IgnoreFilter returns the compiled filter. Always non-nil after
// Validate has succeeded. A zero-pattern filter is a pass-through.
func (c *Config) IgnoreFilter() *ignore.Filter { return c.ignoreFilter }

// Validate reports errors that should prevent the program from running:
// conflicting auth options on the Prometheus or vmalert clients, and
// unrecognized --backend values. The empty string is treated as "auto".
func (c *Config) Validate() error {
	if c.Prometheus.Token != "" && c.Prometheus.BasicAuth != "" {
		return errors.New("config: --prom-token and --prom-basic-auth are mutually exclusive")
	}
	if c.VMAlert.Token != "" && c.VMAlert.BasicAuth != "" {
		return errors.New("config: --vmalert-token and --vmalert-basic-auth are mutually exclusive")
	}
	switch c.Backend {
	case "", "auto", "prometheus", "victoria":
		// ok
	default:
		return fmt.Errorf("config: --backend %q invalid (want auto|prometheus|victoria)", c.Backend)
	}
	sev, enabled, err := findings.ParseFailOn(c.FailOn)
	if err != nil {
		return fmt.Errorf("invalid --fail-on: %w", err)
	}
	c.failOnSeverity = sev
	c.failOnEnabled = enabled
	filter, err := ignore.New(ignore.Patterns{
		Metric: c.Ignore.Metric,
		Label:  c.Ignore.Label,
		Alert:  c.Ignore.Alert,
	})
	if err != nil {
		return fmt.Errorf("invalid --ignore-*: %w", err)
	}
	c.ignoreFilter = filter
	return nil
}
