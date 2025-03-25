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

var (
	cfgFile string
	verbose bool
	
	// Config holds the application configuration
	cfg *config.Config
	
	// Logger for the application
	logger *zap.Logger
)

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
		logger, err = zap.NewDevelopment()
	} else {
		logger, err = zap.NewProduction()
	}
	
	if err != nil {
		fmt.Println("Error initializing logger:", err)
		os.Exit(1)
	}
	
	// Load configuration
	cfg, err = config.Load(cfgFile)
	if err != nil {
		logger.Error("Failed to load configuration", zap.Error(err))
		fmt.Println("Error loading configuration:", err)
		os.Exit(1)
	}
	
	// Override config with command line flags
	if verbose {
		cfg.EnableLogging = true
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	err := RootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
} 