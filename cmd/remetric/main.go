// Package main is the remetric CLI entrypoint.
package main

import (
	"os"

	"github.com/remetric-dev/remetric/internal/cli"
)

var version = "dev"

func main() {
	os.Exit(cli.Execute(version))
}
