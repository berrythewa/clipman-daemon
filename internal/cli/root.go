package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	cmdpkg "github.com/berrythewa/clipman-daemon/internal/cli/cmd"
	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/clipboard"
	"github.com/berrythewa/clipman-daemon/internal/storage"
	"github.com/berrythewa/clipman-daemon/internal/broker"
	"github.com/berrythewa/clipman-daemon/pkg/utils"
	"github.com/spf13/cobra"
)

var (
	// Flags that apply to all commands
	logLevel     string
	deviceID     string
	cfgFile      string
	noFileLog    bool
	detach       bool
	noBroker     bool
	maxSize      int64

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
	Use:   "clipman",
	Short: "Clipman is a clipboard manager",
	Long: `Clipman is a clipboard manager that monitors your clipboard
and provides history, searching, and synchronization capabilities.

Running clipman without any commands starts the daemon in the foreground.
Use --detach to run it in the background.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// If no subcommand is provided, run in daemon mode by default
		return runDaemon()
	},
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

// runDaemon implements the daemon mode functionality (previously in run.go)
func runDaemon() error {
	logger.Info("Starting Clipman daemon")
	
	// If maxSize is specified, override config
	if maxSize > 0 {
		cfg.Storage.MaxSize = maxSize
	}

	// Get all system paths
	paths := cfg.GetPaths()
	
	// If detach flag is set, detach from terminal
	if detach {
		logger.Info("Detaching from terminal and running in background")
		if err := daemonize(); err != nil {
			return fmt.Errorf("failed to daemonize: %v", err)
		}
		// Parent process exits here, child continues
		return nil
	}
	
	// Initialize MQTT client if configured and not explicitly disabled
	var mqttClient broker.MQTTClientInterface
	if !noBroker && cfg.Broker.URL != "" {
		logger.Info("Initializing broker connection", 
			"url", cfg.Broker.URL,
			"device_id", cfg.DeviceID)
		
		var err error
		mqttClient, err = broker.NewMQTTClient(cfg, logger)
		if err != nil {
			logger.Warn("Failed to initialize MQTT client", "error", err)
			logger.Info("Continuing without MQTT support")
		} else {
			logger.Info("MQTT client initialized successfully")
		}
	} else {
		if noBroker {
			logger.Info("MQTT broker disabled by command line flag")
		} else if cfg.Broker.URL == "" {
			logger.Info("No MQTT broker URL configured, running without broker connection")
		}
	}
	
	// Initialize storage
	storageConfig := storage.StorageConfig{
		DBPath:     paths.DBFile,
		MaxSize:    cfg.Storage.MaxSize,
		DeviceID:   cfg.DeviceID,
		Logger:     logger,
		MQTTClient: mqttClient,
	}
	
	logger.Info("Storage configuration", 
		"db_path", paths.DBFile,
		"max_size_bytes", storageConfig.MaxSize,
		"device_id", cfg.DeviceID)
		
	store, err := storage.NewBoltStorage(storageConfig)
	if err != nil {
		logger.Error("Failed to initialize storage", "error", err)
		return err
	}
	defer store.Close()
	
	// Start the monitor
	monitor := clipboard.NewMonitor(cfg, mqttClient, logger, store)
	if err := monitor.Start(); err != nil {
		logger.Error("Failed to start monitor", "error", err)
		return err
	}
	
	logger.Info("Monitor started")
	logger.Info("Running until interrupted, press Ctrl+C to stop")
	
	// Run indefinitely - block until interrupted
	select {}
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
		logger.Info("Shutting down Clipman")
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
	
	// Flags for daemon mode (which is the default command now)
	RootCmd.Flags().BoolVar(&detach, "detach", false, "Detach from terminal and run in background")
	RootCmd.Flags().BoolVar(&noBroker, "no-broker", false, "Disable MQTT broker connection even if configured")
	RootCmd.Flags().Int64Var(&maxSize, "max-size", 0, "Override max cache size in bytes (default 100MB)")
} 