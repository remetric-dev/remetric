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
