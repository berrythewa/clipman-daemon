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
	cli.Execute()
} 