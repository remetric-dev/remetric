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
)

// Config is the fully-resolved runtime configuration.
type Config struct {
	Prometheus PrometheusConfig
	Grafana    GrafanaConfig
	Verbose    bool
	Timeout    time.Duration
	NoColor    bool   `mapstructure:"no_color"`
	Output     string `mapstructure:"output"`
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
	fs.Bool("no-color", false, "Disable colored output")
	fs.String("output", "terminal", "Output format: terminal|json")
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
		"no-color":                "no_color",
		"output":                  "output",
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
