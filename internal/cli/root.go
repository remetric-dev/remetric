package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/remetric-dev/remetric/internal/config"
)

// Args is the input bundle for ExecuteWith — used in tests so we can swap
// stdout/stderr.
type Args struct {
	Version string
	Args    []string
	Stdout  io.Writer
	Stderr  io.Writer
}

// Execute is the default entrypoint used by main.go.
func Execute(version string) int {
	return ExecuteWith(Args{
		Version: version,
		Args:    os.Args[1:],
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
	})
}

// ExecuteWith runs the root command with explicit IO and arguments.
func ExecuteWith(args Args) int {
	root := newRootCmd(args.Version)
	root.SetArgs(args.Args)
	root.SetOut(args.Stdout)
	root.SetErr(args.Stderr)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := root.ExecuteContext(ctx); err != nil {
		var ee *exitError
		if errors.As(err, &ee) {
			return ee.code
		}
		var perr *flagError
		if errors.As(err, &perr) {
			return 2
		}
		// Cobra emits an untyped error for unknown commands; map it to the
		// usage exit code.
		if strings.HasPrefix(err.Error(), "unknown command") {
			return 2
		}
		return 1
	}
	return 0
}

type exitError struct {
	code int
	err  error
}

func (e *exitError) Error() string { return e.err.Error() }
func (e *exitError) Unwrap() error { return e.err }

type flagError struct{ err error }

func (e *flagError) Error() string { return e.err.Error() }

func newRootCmd(version string) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "remetric",
		Short:         "Find waste in Prometheus, Grafana & Loki stacks.",
		SilenceUsage:  true,
		SilenceErrors: false,
		Version:       version,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			cfgPath, _ := cmd.Flags().GetString("config")
			cfg, err := config.Load(cmd.Flags(), cfgPath)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}
			verbose, _ := cmd.Flags().GetBool("verbose")
			lvl := slog.LevelWarn
			if verbose {
				lvl = slog.LevelDebug
			}
			l := slog.New(slog.NewTextHandler(cmd.ErrOrStderr(), &slog.HandlerOptions{Level: lvl}))
			ctx := withLogger(withConfig(cmd.Context(), cfg), l)
			cmd.SetContext(ctx)
			return nil
		},
	}
	cmd.SetFlagErrorFunc(func(_ *cobra.Command, err error) error { return &flagError{err: err} })

	config.BindFlags(cmd.PersistentFlags())

	cmd.AddCommand(newDoctorCmd())
	cmd.AddCommand(newCardinalityCmd())
	return cmd
}

// Stubs replaced by Tasks 8.2 (doctor) and 8.3 (cardinality).
func newDoctorCmd() *cobra.Command {
	return &cobra.Command{Use: "doctor", Short: "Check connectivity", RunE: func(_ *cobra.Command, _ []string) error { return nil }}
}

func newCardinalityCmd() *cobra.Command {
	cardinality := &cobra.Command{Use: "cardinality", Short: "Cardinality analysis"}
	cardinality.AddCommand(&cobra.Command{Use: "top", Short: "Top cardinality offenders", RunE: func(_ *cobra.Command, _ []string) error { return nil }})
	return cardinality
}
