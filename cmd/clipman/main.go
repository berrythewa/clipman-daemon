package main

import (
	"fmt"
	"os"

	"github.com/berrythewa/clipman-daemon/internal/cli"
)

var (
	version   = "dev"
	buildTime = "unknown"
	commit    = "none"
)

func main() {
	// Set version information
	cli.SetVersionInfo(version, buildTime, commit)

	// Execute the root command
	if err := cli.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
} 