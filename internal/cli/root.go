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
	"github.com/berrythewa/clipman-daemon/internal/platform"
	"github.com/berrythewa/clipman-daemon/internal/types"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// Flags that apply to all commands
	logLevel     string
	deviceID     string
	cfgFile      string
	noFileLog    bool
	detach       bool
	noSync       bool
	maxSize      int64

	// The loaded configuration
	cfg *config.Config

	// Logger instance
	zapLogger *zap.Logger
	
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
		return runDaemon(cmd, args)
	},
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		var err error

		// Load config first
		cfg, err = config.Load(cfgFile)
		if err != nil {
			return fmt.Errorf("failed to load config: %v", err)
		}

		// Override config with flags
		if logLevel != "" {
			// Set log level in config
			cfg.Log.Format = logLevel
		}

		if deviceID != "" {
			cfg.DeviceID = deviceID
		}

		// Initialize the logger
		zapConfig := zap.NewProductionConfig()
		
		// Set log level based on config or flags
		level := getZapLogLevel(cfg.Log.Format)
		zapConfig.Level = zap.NewAtomicLevelAt(level)
		
		// Enable development mode for better console output if in debug mode
		if level == zapcore.DebugLevel {
			zapConfig.Development = true
			zapConfig.Encoding = "console"
		}
		
		// Configure file logging
		if cfg.Log.EnableFileLogging && !noFileLog {
			// Get system paths
			paths := cfg.GetPaths()
			
			// Set up file logging
			zapConfig.OutputPaths = append(zapConfig.OutputPaths, fmt.Sprintf("%s/clipman.log", paths.LogDir))
			zapConfig.ErrorOutputPaths = append(zapConfig.ErrorOutputPaths, fmt.Sprintf("%s/clipman_error.log", paths.LogDir))
			
			// Use MB value from config
			maxLogSizeMB := int(cfg.Log.MaxLogSize / (1024 * 1024))
			if maxLogSizeMB <= 0 {
				maxLogSizeMB = 10 // Default to 10MB
			}
			
			// Configure rotation
			zapConfig.DisableStacktrace = false
			zapConfig.DisableCaller = false
		}
		
		// Create the logger
		zapLogger, err = zapConfig.Build()
		if err != nil {
			return fmt.Errorf("failed to initialize logger: %v", err)
		}
		
		// Set up signal handlers for graceful shutdown
		setupSignalHandlers()
		
		// Log startup information
		zapLogger.Debug("Configuration loaded", 
			zap.String("log_level", cfg.Log.Format),
			zap.String("device_id", cfg.DeviceID),
			zap.String("data_dir", cfg.SystemPaths.DataDir))
		
		if cfg.Log.EnableFileLogging && !noFileLog {
			zapLogger.Info("File logging enabled", 
				zap.String("log_dir", cfg.SystemPaths.LogDir),
				zap.Int("max_size_mb", int(cfg.Log.MaxLogSize / (1024 * 1024))),
				zap.Int("max_files", cfg.Log.MaxLogFiles))
		} else {
			zapLogger.Info("File logging disabled")
		}
			
		// Share cfg and logger with cmd package
		cmdpkg.SetConfig(cfg)
		cmdpkg.SetZapLogger(zapLogger)

		return nil
	},
}

// getZapLogLevel converts string log level to zap.Level
func getZapLogLevel(level string) zapcore.Level {
	switch level {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	default:
		return zapcore.InfoLevel // Default to info level
	}
}

// runDaemon implements the daemon mode functionality (previously in run.go)
func runDaemon(cmd *cobra.Command, args []string) error {
	// Check if we're already running as a daemon by checking env vars and process info
	daemonizer := platform.NewDaemonizer()
	isRunningAsDaemon := os.Getenv("CLIPMAN_DAEMON") == "1"
	isPidSessionLeader := daemonizer.IsRunningAsDaemon()
	pid := os.Getpid()
	ppid := os.Getppid()
	
	zapLogger.Debug("Daemon status check", 
		zap.Bool("env_var_set", isRunningAsDaemon),
		zap.Bool("is_session_leader", isPidSessionLeader),
		zap.Int("pid", pid),
		zap.Int("ppid", ppid))
	
	// Handle detach request if needed
	if detach && !isRunningAsDaemon {
		zapLogger.Info("Detaching from terminal and running in background")
		
		// Get the executable path
		executable, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to get executable path: %w", err)
		}
		
		// Get the current working directory
		workDir, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get working directory: %w", err)
		}
		
		// Call the platform-specific daemonizer
		pid, err := daemonizer.Daemonize(executable, os.Args[1:], workDir, cfg.SystemPaths.DataDir)
		if err != nil {
			return fmt.Errorf("failed to daemonize: %w", err)
		}
		
		fmt.Printf("Clipman started in background (PID: %d)\n", pid)
		return nil
	}
	
	if isRunningAsDaemon {
		zapLogger.Info("Running as daemon process", 
			zap.Int("pid", pid),
			zap.Int("ppid", ppid),
			zap.Bool("is_session_leader", isPidSessionLeader))
	} else {
		zapLogger.Info("Running in foreground")
	}
	
	// Continue running as a regular foreground process or daemon child
	zapLogger.Info("Starting Clipman daemon")
	
	// If maxSize is specified, override config
	if maxSize > 0 {
		cfg.Storage.MaxSize = maxSize
	}

	// Get all system paths
	paths := cfg.GetPaths()
	
	// Initialize sync client if configured and not explicitly disabled
	var syncClient sync.SyncClient
	if !noSync && cfg.Sync.URL != "" {
		zapLogger.Info("Initializing sync connection", 
			zap.String("url", cfg.Sync.URL),
			zap.String("device_id", cfg.DeviceID),
			zap.String("mode", cfg.Sync.Mode))
		
		var err error
		syncClient, err = sync.CreateClient(cfg, zapLogger)
		if err != nil {
			zapLogger.Warn("Failed to initialize sync client", zap.Error(err))
			zapLogger.Info("Continuing without sync support")
		} else {
			zapLogger.Info("Sync client initialized successfully")
			
			// If a default group is set, try to join it
			if cfg.Sync.DefaultGroup != "" && cfg.Sync.AutoJoinGroups {
				if err := syncClient.JoinGroup(cfg.Sync.DefaultGroup); err != nil {
					zapLogger.Error("Failed to join default group", 
						zap.String("group", cfg.Sync.DefaultGroup), 
						zap.Error(err))
				} else {
					zapLogger.Info("Joined default group", 
						zap.String("group", cfg.Sync.DefaultGroup))
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
					zapLogger.Error("Failed to set content filter", zap.Error(err))
				}
			}
		}
	} else {
		if noSync {
			zapLogger.Info("Sync client disabled by command line flag")
		} else if cfg.Sync.URL == "" {
			zapLogger.Info("No sync URL configured, running without sync connection")
		}
	}
	
	// Initialize storage
	storageConfig := storage.StorageConfig{
		DBPath:   paths.DBFile,
		MaxSize:  cfg.Storage.MaxSize,
		DeviceID: cfg.DeviceID,
		Logger:   zapLogger,
	}
	
	zapLogger.Info("Storage configuration", 
		zap.String("db_path", paths.DBFile),
		zap.Int64("max_size_bytes", storageConfig.MaxSize),
		zap.String("device_id", cfg.DeviceID))
		
	store, err := storage.NewBoltStorage(storageConfig)
	if err != nil {
		zapLogger.Error("Failed to initialize storage", zap.Error(err))
		return err
	}
	defer store.Close()
	
	// Start the monitor
	monitor := clipboard.NewMonitor(cfg, syncClient, zapLogger, store)
	if err := monitor.Start(); err != nil {
		zapLogger.Error("Failed to start monitor", zap.Error(err))
		return err
	}
	
	zapLogger.Info("Monitor started")
	zapLogger.Info("Running until interrupted, press Ctrl+C to stop")
	
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
	if zapLogger != nil {
		zapLogger.Info("Shutting down Clipman")
		zapLogger.Sync() // Flush any buffered log entries
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
	RootCmd.Flags().BoolVar(&noSync, "no-sync", false, "Disable sync connection even if configured")
	RootCmd.Flags().Int64Var(&maxSize, "max-size", 0, "Override max cache size in bytes (default 100MB)")
	
	// For backward compatibility, keep the old flag but refer to the new one
	RootCmd.PersistentFlags().BoolVar(&noSync, "no-broker", false, "Disable sync connection even if configured (deprecated, use --no-sync)")
} 