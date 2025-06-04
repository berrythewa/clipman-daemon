package main

import (
	"fmt"
	"os"

	"github.com/berrythewa/clipman-daemon/internal/daemon"
)

// Version information - these can be set during build using -ldflags
var (
	version   = "dev"
	buildTime = "unknown"
	commit    = "none"
)

func main() {
	// Check command line arguments for foreground mode
	foreground := false
	for _, arg := range os.Args {
		if arg == "--foreground" {
			foreground = true
			break
		}
	}
	
	// Run the daemon
	var err error
	if foreground {
		err = daemon.RunForeground()
	} else {
		err = daemon.Start()
	}
	
	if err != nil {
		fmt.Fprintf(os.Stderr, "Daemon error: %v\n", err)
		os.Exit(1)
	}
}