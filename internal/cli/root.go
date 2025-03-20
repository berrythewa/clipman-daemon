package cli

import (
	"fmt"
	"os"

	cmdpkg "github.com/berrythewa/clipman-daemon/internal/cli/cmd"
	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/pkg/utils"
	"github.com/spf13/cobra"
)

var (
	// Flags that apply to all commands
	logLevel    string
	deviceID    string
	cfgFile     string

	// The loaded configuration
	cfg *config.Config

	// Logger instance
	logger *utils.Logger
	
	// Version information - set by main
	Version   = "dev"
	BuildTime = "unknown"
	Commit    = "none"
)

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "clipmand",
	Short: "Clipman is a clipboard manager daemon",
	Long: `Clipman is a clipboard manager daemon that monitors your clipboard
and provides history, searching, and synchronization capabilities.

It can be run as a background service or interactively for management
and inspection of clipboard history.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error

		// Load config first
		cfg, err = config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %v", err)
		}

		// Override config with flags
		if logLevel != "" {
			cfg.LogLevel = logLevel
		}

		if deviceID != "" {
			cfg.DeviceID = deviceID
		}

		// Initialize the logger
		logger = utils.NewLogger(cfg.LogLevel, nil)
		logger.Debug("Configuration loaded", 
			"log_level", cfg.LogLevel,
			"device_id", cfg.DeviceID,
			"data_dir", cfg.DataDir)
			
		// Share cfg and logger with cmd package
		cmdpkg.SetConfig(cfg)
		cmdpkg.SetLogger(logger)

		return nil
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

// SetVersionInfo sets the version information used by the version command
func SetVersionInfo(version, buildTime, commit string) {
	Version = version
	BuildTime = buildTime
	Commit = commit
}

// AddCommand adds a command to the root command
func AddCommand(cmd *cobra.Command) {
	RootCmd.AddCommand(cmd)
}

func init() {
	// Global flags for all commands
	RootCmd.PersistentFlags().StringVar(&logLevel, "log-level", "", "Log level (debug, info, warn, error)")
	RootCmd.PersistentFlags().StringVar(&deviceID, "device-id", "", "Override device ID")
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.clipman/config.json)")
} 