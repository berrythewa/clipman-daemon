package daemon

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/clipboard"
	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/ipc"
	"github.com/berrythewa/clipman-daemon/internal/p2p"
	"github.com/berrythewa/clipman-daemon/internal/storage"
	"github.com/berrythewa/clipman-daemon/internal/platform"
	"github.com/berrythewa/clipman-daemon/internal/types"
	"go.uber.org/zap"
)

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
	d.logger.Info("Storage initialized successfully")

	// Initialize clipboard monitor
	d.logger.Info("Initializing clipboard...")
	clipboard := clipboard.NewClipboard()
	d.logger.Info("Clipboard NewClipboard() called", zap.Bool("is_nil", clipboard == nil))
	d.clipboard = clipboard
	d.logger.Info("Clipboard assigned to daemon", zap.Bool("daemon_clipboard_is_nil", d.clipboard == nil))

	// Initialize sync if enabled
	if d.cfg.Sync.Enabled {
		syncNode, err := p2p.NewNode(d.ctx, d.cfg, d.logger)
		if err != nil {
			return fmt.Errorf("failed to initialize sync: %w", err)
		}
		d.sync = syncNode
	}

	// Initialize IPC handler
	d.ipc = d.handleIPCRequest

	d.logger.Info("All daemon components initialized successfully")
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
		daemonizer := platform.GetPlatformDaemonizer()
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

	// Debug logging before clipboard operations
	d.logger.Info("About to start clipboard monitor", zap.Bool("clipboard_is_nil", d.clipboard == nil))

	// Start clipboard monitor
	contentCh := make(chan *types.ClipboardContent)
	stopCh := make(chan struct{})
	d.logger.Info("Created channels, about to call MonitorChanges")
	go d.clipboard.MonitorChanges(contentCh, stopCh)
	d.logger.Info("MonitorChanges goroutine started")

	// Start content processing loop - THIS WAS MISSING!
	go func() {
		for {
			select {
			case content := <-contentCh:
				// Save content to storage with hash generation
				if err := d.storage.SaveContent(content); err != nil {
					d.logger.Error("Failed to save clipboard content to storage", zap.Error(err))
				} else {
					d.logger.Info("Saved clipboard content to storage", 
						zap.String("type", string(content.Type)),
						zap.String("hash", content.Hash),
						zap.Int("size", len(content.Data)))
				}
			case <-d.ctx.Done():
				return
			}
		}
	}()

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

	// Signal clipboard monitoring to stop
	close(stopCh)

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
func (d *Daemon) handleIPCRequest(req *ipc.Request) *ipc.Response {
	d.logger.Debug("Received IPC request", zap.String("command", req.Command))
	
	switch req.Command {
	case "history", "history.list":
		return d.handleHistoryListRequest(req)
	case "history.show":
		return d.handleHistoryShowRequest(req)
	case "history.delete":
		return d.handleHistoryDeleteRequest(req)
	case "history.stats":
		return d.handleHistoryStatsRequest(req)
	case "clip.get":
		return d.handleClipGetRequest(req)
	case "clip.set":
		return d.handleClipSetRequest(req)
	case "clip.watch":
		return d.handleClipWatchRequest(req)
	case "clip.flush":
		return d.handleClipFlushRequest(req)
	default:
		return &ipc.Response{
			Status:  "error", 
			Message: fmt.Sprintf("Unknown command: %s", req.Command),
		}
	}
}

// handleHistoryListRequest handles history list requests using daemon's storage
func (d *Daemon) handleHistoryListRequest(req *ipc.Request) *ipc.Response {
	d.logger.Debug("Processing history list request")

	// Parse request arguments
	limit := int64(10) // default
	if l, ok := req.Args["limit"].(float64); ok {
		limit = int64(l)
	}
	
	reverse := false
	if r, ok := req.Args["reverse"].(bool); ok {
		reverse = r
	}
	
	contentType := ""
	if t, ok := req.Args["type"].(string); ok {
		contentType = t
	}

	// Build history options
	options := config.HistoryOptions{
		Limit:       limit,
		Reverse:     reverse,
		ContentType: types.ContentType(contentType),
	}
	
	// Parse time-based filters
	if since, ok := req.Args["since"].(string); ok {
		if sinceTime, err := time.Parse(time.RFC3339, since); err == nil {
			options.Since = sinceTime
		}
	}
	
	if before, ok := req.Args["before"].(string); ok {
		if beforeTime, err := time.Parse(time.RFC3339, before); err == nil {
			options.Before = beforeTime
		}
	}

	// Use daemon's existing storage instance - NO new DB connections!
	contents, err := d.storage.GetHistory(options)
	if err != nil {
		d.logger.Error("Failed to get history from storage", zap.Error(err))
		return &ipc.Response{
			Status:  "error",
			Message: fmt.Sprintf("Failed to get history: %v", err),
		}
	}

	d.logger.Debug("Retrieved history", 
		zap.Int("count", len(contents)),
		zap.Int64("limit", limit),
		zap.Bool("reverse", reverse))

	return &ipc.Response{
		Status: "ok",
		Data:   contents,
	}
}

// handleHistoryShowRequest handles showing specific history entry
func (d *Daemon) handleHistoryShowRequest(req *ipc.Request) *ipc.Response {
	return &ipc.Response{
		Status:  "error",
		Message: "history.show not implemented yet",
	}
}

// handleHistoryDeleteRequest handles deleting history entries
func (d *Daemon) handleHistoryDeleteRequest(req *ipc.Request) *ipc.Response {
	return &ipc.Response{
		Status:  "error",
		Message: "history.delete not implemented yet",
	}
}

// handleHistoryStatsRequest handles history statistics request
func (d *Daemon) handleHistoryStatsRequest(req *ipc.Request) *ipc.Response {
	return &ipc.Response{
		Status:  "error",
		Message: "history.stats not implemented yet",
	}
}

// handleClipGetRequest handles getting current clipboard content
func (d *Daemon) handleClipGetRequest(req *ipc.Request) *ipc.Response {
	d.logger.Debug("Processing clip get request")

	// Use daemon's clipboard instance
	content, err := d.clipboard.Read()
	if err != nil {
		d.logger.Error("Failed to read clipboard", zap.Error(err))
		return &ipc.Response{
			Status:  "error",
			Message: fmt.Sprintf("Failed to read clipboard: %v", err),
		}
	}

	return &ipc.Response{
		Status: "ok",
		Data:   content,
	}
}

// handleClipSetRequest handles setting clipboard content
func (d *Daemon) handleClipSetRequest(req *ipc.Request) *ipc.Response {
	d.logger.Debug("Processing clip set request")

	// Parse content from request
	contentData, ok := req.Args["content"]
	if !ok {
		return &ipc.Response{
			Status:  "error",
			Message: "No content provided",
		}
	}

	// Convert to ClipboardContent
	var content *types.ClipboardContent
	if contentMap, ok := contentData.(map[string]interface{}); ok {
		// Parse from map
		data, _ := contentMap["data"].(string)
		contentType, _ := contentMap["type"].(string)
		
		content = &types.ClipboardContent{
			Type: types.ContentType(contentType),
			Data: []byte(data),
		}
	} else {
		return &ipc.Response{
			Status:  "error",
			Message: "Invalid content format",
		}
	}

	// Use daemon's clipboard instance
	err := d.clipboard.Write(content)
	if err != nil {
		d.logger.Error("Failed to write clipboard", zap.Error(err))
		return &ipc.Response{
			Status:  "error",
			Message: fmt.Sprintf("Failed to write clipboard: %v", err),
		}
	}

	return &ipc.Response{
		Status:  "ok",
		Message: "Clipboard content set successfully",
	}
}

// handleClipWatchRequest handles watching clipboard changes
func (d *Daemon) handleClipWatchRequest(req *ipc.Request) *ipc.Response {
	return &ipc.Response{
		Status:  "error",
		Message: "clip.watch not implemented yet - use daemon monitoring instead",
	}
}

// handleClipFlushRequest handles flushing clipboard history
func (d *Daemon) handleClipFlushRequest(req *ipc.Request) *ipc.Response {
	d.logger.Debug("Processing clip flush request")

	// Use daemon's storage instance to flush
	err := d.storage.FlushCache()
	if err != nil {
		d.logger.Error("Failed to flush clipboard history", zap.Error(err))
		return &ipc.Response{
			Status:  "error",
			Message: fmt.Sprintf("Failed to flush history: %v", err),
		}
	}

	return &ipc.Response{
		Status:  "ok",
		Message: "Clipboard history flushed successfully",
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