package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

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
	noFileLog   bool

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

		// Get system paths
		paths := cfg.GetPaths()

		// Initialize the logger with file logging
		loggerOptions := utils.LoggerOptions{
			Level:  cfg.LogLevel,
			Output: os.Stdout,
		}
		
		// Enable file logging unless explicitly disabled by flag or config
		enableFileLogging := cfg.Log.EnableFileLogging && !noFileLog
		if enableFileLogging {
			loggerOptions.LogDir = paths.LogDir
			loggerOptions.MaxSize = cfg.Log.MaxLogSize
			loggerOptions.MaxFiles = cfg.Log.MaxLogFiles
		}
		
		logger = utils.NewLogger(loggerOptions)
		
		// Set up signal handlers for graceful shutdown
		setupSignalHandlers()
		
		logger.Debug("Configuration loaded", 
			"log_level", cfg.LogLevel,
			"device_id", cfg.DeviceID,
			"data_dir", cfg.DataDir)
		
		if enableFileLogging {
			logger.Info("File logging enabled", 
				"log_dir", paths.LogDir,
				"max_size", cfg.Log.MaxLogSize,
				"max_files", cfg.Log.MaxLogFiles)
		} else {
			logger.Info("File logging disabled")
		}
			
		// Share cfg and logger with cmd package
		cmdpkg.SetConfig(cfg)
		cmdpkg.SetLogger(logger)

		return nil
	},
}

// setupSignalHandlers sets up handlers for OS signals to perform cleanup
func setupSignalHandlers() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	
	go func() {
		<-c
		fmt.Println("\nShutdown signal received, cleaning up...")
		cleanup()
		os.Exit(0)
	}()
}

// cleanup performs cleanup operations before exit
func cleanup() {
	if logger != nil {
		if err := logger.Close(); err != nil {
			fmt.Fprintf(os.Stderr, "Error closing logger: %v\n", err)
		}
	}
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	defer cleanup() // Ensure cleanup happens after command execution
	
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
	RootCmd.PersistentFlags().BoolVar(&noFileLog, "no-file-log", false, "Disable logging to file")
} 