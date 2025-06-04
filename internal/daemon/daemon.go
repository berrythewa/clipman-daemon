package daemon

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/ipc"
	"github.com/berrythewa/clipman-daemon/internal/clipboard"
	"github.com/berrythewa/clipman-daemon/internal/storage"
	"github.com/berrythewa/clipman-daemon/internal/p2p"
	"github.com/berrythewa/clipman-daemon/internal/types"
	"go.uber.org/zap"
)

// Daemonizer defines the interface for platform-specific daemonization
// Each platform implements this interface for native daemon process management
type Daemonizer interface {
	// Daemonize forks the current process and runs it in the background
	// Returns the PID of the new process or an error
	Daemonize(executable string, args []string, workDir string, dataDir string) (int, error)
	
	// IsRunningAsDaemon returns true if the current process is running as a daemon
	IsRunningAsDaemon() bool
}

// Package variable to hold the platform-specific daemonizer implementation
var (
	defaultDaemonizer Daemonizer
)

// GetPlatformDaemonizer returns the appropriate daemonizer implementation for the current platform
// The actual implementation is selected at compile time through build tags
func GetPlatformDaemonizer() Daemonizer {
	if defaultDaemonizer == nil {
		panic("no daemonizer implementation registered for this platform")
	}
	return defaultDaemonizer
}

// Daemon represents the main daemon process
type Daemon struct {
	ctx    context.Context
	cancel context.CancelFunc
	cfg    *config.Config
	logger *zap.Logger
	
	// Components
	clipboard clipboard.Clipboard
	storage   *storage.BoltStorage
	sync      *p2p.Node
	ipc       func(*ipc.Request) *ipc.Response
}

// NewDaemon creates a new daemon instance
func NewDaemon(cfg *config.Config, logger *zap.Logger) *Daemon {
	ctx, cancel := context.WithCancel(context.Background())
	return &Daemon{
		ctx:    ctx,
		cancel: cancel,
		cfg:    cfg,
		logger: logger,
	}
}

// Initialize sets up all daemon components
func (d *Daemon) Initialize() error {
	d.logger.Info("Initializing daemon components")

	// Initialize storage
	storage, err := storage.NewBoltStorage(storage.StorageConfig{
		DBPath:    d.cfg.Storage.DBPath,
		MaxSize:   d.cfg.Storage.MaxSize,
		DeviceID:  d.cfg.DeviceID,
		Logger:    d.logger,
		KeepItems: d.cfg.Storage.KeepItems,
	})
	if err != nil {
		return fmt.Errorf("failed to initialize storage: %w", err)
	}
	d.storage = storage

	// Initialize clipboard monitor
	clipboard := clipboard.NewClipboard()
	d.clipboard = clipboard

	// Initialize sync if enabled
	if d.cfg.Sync.Enabled {
		syncNode, err := p2p.NewNode(d.ctx, d.cfg, d.logger)
		if err != nil {
			return fmt.Errorf("failed to initialize sync: %w", err)
		}
		d.sync = syncNode
	}

	// Initialize IPC handler
	d.ipc = handleIPCRequest

	return nil
}

// Start launches the daemon process
func Start() error {
	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Setup logging
	logger, err := setupLogger(cfg)
	if err != nil {
		return fmt.Errorf("failed to setup logging: %w", err)
	}

	// Create and initialize daemon
	daemon := NewDaemon(cfg, logger)
	if err := daemon.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize daemon: %w", err)
	}

	// Handle daemonization if needed
	if os.Getenv("CLIPMAN_DAEMON") != "1" {
		// Get platform-specific daemonizer
		daemonizer := GetPlatformDaemonizer()
		executable, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to get executable path: %w", err)
		}

		workDir, _ := os.Getwd()
		dataDir := cfg.SystemPaths.DataDir
		pid, err := daemonizer.Daemonize(executable, []string{}, workDir, dataDir)
		if err != nil {
			return fmt.Errorf("failed to daemonize: %w", err)
		}
		fmt.Printf("Clipman daemon started with PID %d\n", pid)
		return nil
	}

	// Run the daemon
	return daemon.Run()
}

// Run starts all daemon components and handles shutdown
func (d *Daemon) Run() error {
	d.logger.Info("Starting daemon components")

	// Start clipboard monitor
	contentCh := make(chan *types.ClipboardContent)
	stopCh := make(chan struct{})
	go d.clipboard.MonitorChanges(contentCh, stopCh)

	// Start sync if enabled
	if d.sync != nil {
		if err := d.sync.Start(); err != nil {
			return fmt.Errorf("failed to start sync: %w", err)
		}
	}

	// Start IPC server
	go func() {
		if err := ipc.ListenAndServe("", d.ipc); err != nil {
			d.logger.Error("IPC server error", zap.Error(err))
		}
	}()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Wait for shutdown signal
	<-sigChan
	d.logger.Info("Shutdown signal received")

	// Perform graceful shutdown
	return d.Shutdown()
}

// Shutdown gracefully stops all daemon components
func (d *Daemon) Shutdown() error {
	d.logger.Info("Shutting down daemon components")

	// Stop clipboard monitor
	if d.clipboard != nil {
		d.clipboard.Close()
	}

	// Stop sync if enabled
	if d.sync != nil {
		if err := d.sync.Stop(); err != nil {
			d.logger.Error("Failed to stop sync", zap.Error(err))
		}
	}

	// Cancel context
	d.cancel()

	// Close storage
	if d.storage != nil {
		if err := d.storage.Close(); err != nil {
			d.logger.Error("Failed to close storage", zap.Error(err))
		}
	}

	d.logger.Info("Daemon shutdown complete")
	return nil
}

// Kill stops the daemon process using the PID file
func Kill() error {
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	pidFile := fmt.Sprintf("%s/run/clipman.pid", cfg.SystemPaths.DataDir)
	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		return fmt.Errorf("failed to read PID file: %w", err)
	}

	var pid int
	if _, err := fmt.Sscanf(string(pidBytes), "%d", &pid); err != nil || pid <= 0 {
		return fmt.Errorf("invalid PID in file: %s", string(pidBytes))
	}

	proc, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	if err := proc.Signal(syscall.SIGTERM); err != nil {
		return fmt.Errorf("failed to send termination signal: %w", err)
	}

	// Wait for process to exit
	for i := 0; i < 10; i++ {
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			// Process has exited
			os.Remove(pidFile)
			return nil
		}
		time.Sleep(time.Second)
	}

	// Force kill if still running
	if err := proc.Kill(); err != nil {
		return fmt.Errorf("failed to kill process: %w", err)
	}

	os.Remove(pidFile)
	return nil
}

// setupLogger configures the logger based on configuration
func setupLogger(cfg *config.Config) (*zap.Logger, error) {
	config := zap.NewProductionConfig()
	
	// Set log level
	level := zap.InfoLevel
	if cfg.Log.Format == "debug" {
		level = zap.DebugLevel
	}
	config.Level = zap.NewAtomicLevelAt(level)

	// Configure file logging
	if cfg.Log.EnableFileLogging {
		config.OutputPaths = append(config.OutputPaths, 
			fmt.Sprintf("%s/clipman.log", cfg.SystemPaths.LogDir))
		config.ErrorOutputPaths = append(config.ErrorOutputPaths,
			fmt.Sprintf("%s/clipman_error.log", cfg.SystemPaths.LogDir))
	}

	return config.Build()
}

// Status checks if the daemon is running.
func Status() (bool, error) {
	dataDir := os.Getenv("CLIPMAN_DATA_DIR")
	if dataDir == "" {
		dataDir = fmt.Sprintf("%s/.local/share/clipman", os.Getenv("HOME"))
	}
	pidFile := fmt.Sprintf("%s/run/clipman.pid", dataDir)
	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		fmt.Println("Clipman daemon is not running (no PID file found).")
		return false, nil
	}
	var pid int
	_, err = fmt.Sscanf(string(pidBytes), "%d", &pid)
	if err != nil || pid <= 0 {
		fmt.Println("Clipman daemon is not running (invalid PID file).")
		return false, nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		fmt.Println("Clipman daemon is not running (process not found).")
		return false, nil
	}
	if err := proc.Signal(os.Interrupt); err != nil {
		fmt.Println("Clipman daemon is not running (process not alive).")
		return false, nil
	}
	fmt.Printf("Clipman daemon is running with PID %d.\n", pid)
	return true, nil
}

// handleIPCRequest processes incoming IPC requests from the CLI.
func handleIPCRequest(req *ipc.Request) *ipc.Response {
	switch req.Command {
	case "history":
		// TODO: Implement actual history retrieval
		return &ipc.Response{Status: "ok", Data: "history not implemented"}
	case "flush":
		// TODO: Implement actual flush
		return &ipc.Response{Status: "ok", Message: "Flush not implemented"}
	default:
		return &ipc.Response{Status: "error", Message: "Unknown command"}
	}
}

// RunForeground runs the daemon in the foreground
func RunForeground() error {
	// Load configuration
	cfg, err := config.Load("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// Setup logging
	logger, err := setupLogger(cfg)
	if err != nil {
		return fmt.Errorf("failed to setup logging: %w", err)
	}

	// Create and initialize daemon
	daemon := NewDaemon(cfg, logger)
	if err := daemon.Initialize(); err != nil {
		return fmt.Errorf("failed to initialize daemon: %w", err)
	}

	// Run the daemon
	return daemon.Run()
}

// Implement a locking mechanism using a PID file
func ensureSingleInstance(cfg *config.Config) error {
	pidFile := fmt.Sprintf("%s/run/clipman.pid", cfg.SystemPaths.DataDir)
	pidBytes, err := os.ReadFile(pidFile)
	if err == nil {
		var pid int
		if _, err := fmt.Sscanf(string(pidBytes), "%d", &pid); err == nil && pid > 0 {
			proc, err := os.FindProcess(pid)
			if err == nil && proc.Signal(syscall.Signal(0)) == nil {
				return fmt.Errorf("daemon already running with PID %d", pid)
			}
		}
	}

	// Write the current process ID to the PID file
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", os.Getpid())), 0644); err != nil {
		return fmt.Errorf("failed to write PID file: %w", err)
	}

	return nil
} 