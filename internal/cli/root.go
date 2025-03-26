package cli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	cmdpkg "github.com/berrythewa/clipman-daemon/internal/cli/cmd"
	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/clipboard"
	"github.com/berrythewa/clipman-daemon/internal/storage"
	"github.com/berrythewa/clipman-daemon/internal/sync"
	"github.com/berrythewa/clipman-daemon/pkg/utils"
	"github.com/berrythewa/clipman-daemon/internal/platform"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
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
		
		// Get the platform-specific daemonizer
		daemonizer := platform.GetPlatformDaemonizer()
		
		// Get the executable path
		executable, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to get executable path: %v", err)
		}
		
		// Get current working directory
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %v", err)
		}
		
		// Daemonize the process
		pid, err := daemonizer.Daemonize(executable, os.Args, cwd, paths.DataDir)
		if err != nil {
			return fmt.Errorf("failed to daemonize: %v", err)
		}
		
		fmt.Printf("Clipman started in background (PID: %d)\n", pid)
		
		// Parent process exits here, child continues
		return nil
	}
	
	// Initialize sync client if configured and not explicitly disabled
	var syncClient sync.SyncClient
	if !noBroker && (cfg.Sync.URL != "" || cfg.Broker.URL != "") {
		url := cfg.Sync.URL
		if url == "" {
			url = cfg.Broker.URL
		}
		
		logger.Info("Initializing sync connection", 
			"url", url,
			"device_id", cfg.DeviceID,
			"mode", cfg.Sync.Mode)
		
		var err error
		syncClient, err = sync.CreateClient(cfg, logger)
		if err != nil {
			logger.Warn("Failed to initialize sync client", "error", err)
			logger.Info("Continuing without sync support")
		} else {
			logger.Info("Sync client initialized successfully")
			
			// If a default group is set, try to join it
			if cfg.Sync.DefaultGroup != "" && cfg.Sync.AutoJoinGroups {
				if err := syncClient.JoinGroup(cfg.Sync.DefaultGroup); err != nil {
					logger.Error("Failed to join default group", 
						"group", cfg.Sync.DefaultGroup, 
						"error", err)
				} else {
					logger.Info("Joined default group", 
						"group", cfg.Sync.DefaultGroup)
				}
			}
			
			// Set up content filtering based on config
			if len(cfg.Sync.AllowedTypes) > 0 || len(cfg.Sync.ExcludedTypes) > 0 || cfg.Sync.MaxSyncSize > 0 {
				contentTypes := make([]types.ContentType, 0, len(cfg.Sync.AllowedTypes))
				for _, t := range cfg.Sync.AllowedTypes {
					contentTypes = append(contentTypes, types.ContentType(t))
				}
				
				excludedTypes := make([]types.ContentType, 0, len(cfg.Sync.ExcludedTypes))
				for _, t := range cfg.Sync.ExcludedTypes {
					excludedTypes = append(excludedTypes, types.ContentType(t))
				}
				
				filter := &sync.ContentFilter{
					AllowedTypes:    contentTypes,
					ExcludedTypes:   excludedTypes,
					MaxSize:         cfg.Sync.MaxSyncSize,
					IncludePatterns: cfg.Sync.IncludePatterns,
					ExcludePatterns: cfg.Sync.ExcludePatterns,
				}
				
				if err := syncClient.SetContentFilter(filter); err != nil {
					logger.Error("Failed to set content filter", "error", err)
				}
			}
		}
	} else {
		if noBroker {
			logger.Info("Sync client disabled by command line flag")
		} else if cfg.Sync.URL == "" && cfg.Broker.URL == "" {
			logger.Info("No sync URL configured, running without sync connection")
		}
	}
	
	// Initialize storage
	storageConfig := storage.StorageConfig{
		DBPath:     paths.DBFile,
		MaxSize:    cfg.Storage.MaxSize,
		DeviceID:   cfg.DeviceID,
		Logger:     logger,
		MQTTClient: syncClient,
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
	monitor := clipboard.NewMonitor(cfg, syncClient, logger, store)
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

// IsRunningAsDaemon returns true if the current process is running as a daemon
func IsRunningAsDaemon() bool {
	daemonizer := platform.GetPlatformDaemonizer()
	return daemonizer.IsRunningAsDaemon()
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