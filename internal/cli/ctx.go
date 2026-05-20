// Package cli is the cobra wiring layer for the remetric binary.
package cli

import (
	"context"
	"log/slog"

	"github.com/remetric-dev/remetric/internal/config"
)

type ctxKey int

const (
	configKey ctxKey = iota
	loggerKey
)

func withConfig(ctx context.Context, c *config.Config) context.Context {
	return context.WithValue(ctx, configKey, c)
}

func configFrom(ctx context.Context) *config.Config {
	c, _ := ctx.Value(configKey).(*config.Config)
	return c
}

func withLogger(ctx context.Context, l *slog.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, l)
}

func loggerFrom(ctx context.Context) *slog.Logger {
	l, _ := ctx.Value(loggerKey).(*slog.Logger)
	if l == nil {
		return slog.Default()
	}
	return l
}
