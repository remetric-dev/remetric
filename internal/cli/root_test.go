// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Andrei Taranik

package cli_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/remetric-dev/remetric/internal/cli"
)

func TestRoot_VersionFlag(t *testing.T) {
	var out bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test-1.0",
		Args:    []string{"--version"},
		Stdout:  &out,
		Stderr:  &out,
	})
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "test-1.0") {
		t.Errorf("output = %q, want to contain version", out.String())
	}
}

func TestRoot_HelpFlag(t *testing.T) {
	var out bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args:    []string{"--help"},
		Stdout:  &out,
		Stderr:  &out,
	})
	if code != 0 {
		t.Errorf("exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "doctor") {
		t.Errorf("help missing doctor: %s", out.String())
	}
	if !strings.Contains(out.String(), "cardinality") {
		t.Errorf("help missing cardinality: %s", out.String())
	}
}

func TestRoot_UnknownCommand(t *testing.T) {
	var out bytes.Buffer
	code := cli.ExecuteWith(cli.Args{
		Version: "test",
		Args:    []string{"bogus"},
		Stdout:  &out,
		Stderr:  &out,
	})
	if code != 2 {
		t.Errorf("exit = %d, want 2", code)
	}
}
