package cmd

import (
	"fmt"
	"os"

	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "clipman",
	Short: "A clipboard manager for the command line",
	Long: `Clipman is a cross-platform clipboard manager daemon
that runs in the background and syncs clipboard contents between devices.`,
}

func init() {
	cobra.OnInitialize(initConfig)

	// Add flags
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.clipman/config.json)")
	RootCmd.PersistentFlags().BoolVar(&verbose, "verbose", false, "enable verbose output")
}

// initConfig initializes the configuration
func initConfig() {
	var err error

	// Initialize logger
	if verbose {
		zapLogger, err = zap.NewDevelopment()
	} else {
		zapLogger, err = zap.NewProduction()
	}
	defer zapLogger.Sync() // Will flush before main exits
	
	if err != nil {
		fmt.Println("Error initializing logger:", err)
		os.Exit(1)
	}
	
	// Load configuration
	cfg, err = config.Load(cfgFile)
	if err != nil {
		
		zapLogger.Error("Failed to load configuration", zap.Error(err))
		fmt.Println("Error loading configuration:", err)
		os.Exit(1)
	}
	
	// Override config with command line flags
	if verbose {
		cfg.EnableLogging = true
	}
	
	// Initialize root commands
	// Child commands are added in their respective init() functions
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
} 