package main

import (
	"flag"
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
	// Define command line flags
	var (
		foreground = flag.Bool("foreground", false, "run in foreground mode")
		logLevel   = flag.String("log-level", "", "log level (debug, info, warn, error)")
		configFile = flag.String("config", "", "config file path")
		help       = flag.Bool("help", false, "show help")
		versionFlag = flag.Bool("version", false, "show version")
	)
	
	// Parse command line arguments
	flag.Parse()
	
	// Handle help
	if *help {
		fmt.Printf("Clipman Daemon %s\n", version)
		fmt.Println("\nUsage:")
		flag.PrintDefaults()
		os.Exit(0)
	}
	
	// Handle version
	if *versionFlag {
		fmt.Printf("Clipman Daemon %s\n", version)
		fmt.Printf("Build time: %s\n", buildTime)
		fmt.Printf("Commit: %s\n", commit)
		os.Exit(0)
	}
	
	// Set log level environment variable if specified
	if *logLevel != "" {
		os.Setenv("CLIPMAN_LOG_LEVEL", *logLevel)
	}
	
	// Set config file environment variable if specified
	if *configFile != "" {
		os.Setenv("CLIPMAN_CONFIG_FILE", *configFile)
	}
	
	// Run the daemon
	var err error
	if *foreground {
		fmt.Printf("Starting Clipman Daemon %s in foreground mode...\n", version)
		err = daemon.RunForeground()
	} else {
		fmt.Printf("Starting Clipman Daemon %s...\n", version)
		err = daemon.Start()
	}
	
	if err != nil {
		fmt.Fprintf(os.Stderr, "Daemon error: %v\n", err)
		os.Exit(1)
	}
}