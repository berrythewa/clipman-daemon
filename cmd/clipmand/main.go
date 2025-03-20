package main

import (
	"github.com/berrythewa/clipman-daemon/internal/cli"
)

// Version information - these can be set during build using -ldflags
var (
	version   = "dev"
	buildTime = "unknown"
	commit    = "none"
)

func main() {
	// Set version information
	cli.SetVersionInfo(version, buildTime, commit)
	
	// Execute the CLI
	cli.Execute()
}