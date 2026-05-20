// Package main is the remetric CLI entrypoint.
package main

import (
	"fmt"
	"os"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	fmt.Fprintln(os.Stderr, "remetric", version, "(scaffolding — CLI not wired yet)")
	os.Exit(0)
}
