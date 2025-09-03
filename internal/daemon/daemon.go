package daemon

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
	"strconv"
	// "encoding/json"

	"github.com/berrythewa/clipman-daemon/internal/clipboard"
	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/ipc"
	"github.com/berrythewa/clipman-daemon/internal/p2p"
	"github.com/berrythewa/clipman-daemon/internal/storage"
	"github.com/berrythewa/clipman-daemon/internal/platform"
	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/berrythewa/clipman-daemon/internal/common"
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
	storage   storage.Storage
	sync      *p2p.Node
	ipc       func(*ipc.Request) *ipc.Response

	// Monitoring channels
	stopCh chan struct{}

	// Supervision
	contentCh               chan *types.ClipboardContent
	supervisionTicker       *time.Ticker
	lastMonitoringCheck     time.Time
	monitoringRestartCount  int
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
	d.logger.Info("🔧 Initializing daemon components")

	// Initialize BoltDB storage directly (simplified for MVP)
	d.logger.Info("📦 Initializing BoltDB storage...", 
		zap.String("db_path", d.cfg.Storage.DBPath))
	
	// Create storage configuration
	storageConfig := storage.StorageConfig{
		DBPath:   d.cfg.Storage.DBPath,
		Logger:   d.logger,
		DeviceID: d.cfg.DeviceID,
	}
	
	// Create storage instance directly
	storageInstance, err := storage.NewBoltStorage(storageConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize BoltDB storage: %w", err)
	}
	d.storage = storageInstance
	d.logger.Info("✅ BoltDB storage initialized successfully")

	// Initialize clipboard
	d.logger.Info("📋 Initializing clipboard...")
	clipboard := clipboard.NewClipboardWithFullConfig(d.logger, d.cfg)
	d.logger.Info("📋 Clipboard NewClipboardWithFullConfig() called",
		zap.Bool("is_nil", clipboard == nil),
		zap.String("clipboard_type", fmt.Sprintf("%T", clipboard)),
		zap.Bool("stealth_mode", d.cfg.StealthMode),
		zap.Bool("html_extract_text", d.cfg.HTML.ExtractText))
	d.clipboard = clipboard
	d.logger.Info("📋 Clipboard assigned to daemon",
		zap.Bool("daemon_clipboard_is_nil", d.clipboard == nil))

	// Initialize sync if enabled
	if d.cfg.Sync.Enabled {
		d.logger.Info("🔄 Initializing sync component...")
		syncNode, err := p2p.NewNode(d.ctx, d.cfg, d.logger)
		if err != nil {
			return fmt.Errorf("failed to initialize sync: %w", err)
		}
		d.sync = syncNode
		d.logger.Info("✅ Sync component initialized")
	}

	// Initialize IPC handler
	d.ipc = d.handleIPCRequest
	d.logger.Info("✅ IPC handler initialized")

	d.logger.Info("✅ All daemon components initialized successfully")
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
	d.logger.Info("🚀 Starting daemon components")

	// Debug logging before clipboard operations
	d.logger.Info("🔍 About to start clipboard monitor",
		zap.Bool("clipboard_is_nil", d.clipboard == nil),
		zap.String("clipboard_type", fmt.Sprintf("%T", d.clipboard)))

	// Initialize supervision
	d.contentCh = make(chan *types.ClipboardContent, 10) // Buffered channel for reliability
	d.stopCh = make(chan struct{})
	d.supervisionTicker = time.NewTicker(30 * time.Second) // Check every 30 seconds
	d.lastMonitoringCheck = time.Now()

	d.logger.Info("Created channels and supervision, about to call MonitorChanges")
	go d.clipboard.MonitorChanges(d.contentCh, d.stopCh)
	d.logger.Info("MonitorChanges goroutine started")

	// Start content processing loop
	go func() {
		d.logger.Info("Content processing loop started")
		for {
			select {
			case content := <-d.contentCh:
				d.logger.Info("Received content from clipboard monitor",
					zap.String("type", string(content.Type)),
					zap.Int("size", len(content.Data)))
				// Save content to storage with hash generation
				if err := d.storage.AddContent(content); err != nil {
					d.logger.Error("❌ Failed to save clipboard content to storage", zap.Error(err))
				} else {
					d.logger.Info("✅ Saved clipboard content to storage",
						zap.String("type", string(content.Type)),
						zap.String("hash", content.Hash),
						zap.Int("size", len(content.Data)))
				}
			case <-d.ctx.Done():
				d.logger.Info("Content processing loop shutting down")
				return
			}
		}
	}()

	// Start monitoring supervision loop
	go func() {
		d.logger.Info("Monitoring supervision started")
		for {
			select {
			case <-d.supervisionTicker.C:
				d.checkAndRestartMonitoring()
			case <-d.ctx.Done():
				d.logger.Info("Monitoring supervision shutting down")
				return
			}
		}
	}()

	// Start sync if enabled
	if d.sync != nil {
		d.logger.Info("🔄 Starting sync component")
		if err := d.sync.Start(); err != nil {
			return fmt.Errorf("failed to start sync: %w", err)
		}
		d.logger.Info("✅ Sync component started")
	}

	// Start IPC server
	go func() {
		d.logger.Info("🔄 Starting IPC server")
		if err := ipc.ListenAndServe("", d.ipc); err != nil {
			d.logger.Error("❌ IPC server error", zap.Error(err))
		}
		d.logger.Info("🛑 IPC server stopped")
	}()

	// Setup signal handling
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	d.logger.Info("✅ Signal handling setup complete")

	// Wait for shutdown signal
	d.logger.Info("⏳ Daemon running, waiting for shutdown signal...")
	<-sigChan
	d.logger.Info("🛑 Shutdown signal received")

	// Stop supervision
	if d.supervisionTicker != nil {
		d.supervisionTicker.Stop()
	}

	// Signal clipboard monitoring to stop
	if d.stopCh != nil {
		close(d.stopCh)
		d.logger.Info("Sent stop signal to clipboard monitor")
	}

	// Perform graceful shutdown
	d.logger.Info("Performing graceful shutdown")
	return d.Shutdown()
}

// checkAndRestartMonitoring checks the health of clipboard monitoring and restarts if needed
func (d *Daemon) checkAndRestartMonitoring() {
	status := d.clipboard.GetMonitoringStatus()
	now := time.Now()

	d.logger.Debug("Checking monitoring health",
		zap.Bool("is_running", status.IsRunning),
		zap.String("mode", status.Mode),
		zap.Time("last_activity", status.LastActivity),
		zap.Int("error_count", status.ErrorCount),
		zap.String("last_error", status.LastError))

	// Check if monitoring is unhealthy
	needsRestart := false
	reason := ""

	if !status.IsRunning {
		needsRestart = true
		reason = "monitoring not running"
	} else if status.ErrorCount > 5 {
		needsRestart = true
		reason = fmt.Sprintf("too many errors (%d)", status.ErrorCount)
	} else if now.Sub(status.LastActivity) > 5*time.Minute {
		needsRestart = true
		reason = "no activity for 5 minutes"
	} else if strings.Contains(status.Mode, "failed") {
		needsRestart = true
		reason = "monitoring in failed state"
	}

	if needsRestart {
		d.monitoringRestartCount++
		d.logger.Warn("Clipboard monitoring needs restart",
			zap.String("reason", reason),
			zap.Int("restart_count", d.monitoringRestartCount),
			zap.String("current_mode", status.Mode))

		// Try to restart monitoring
		if err := d.clipboard.RestartMonitoring(d.contentCh, d.stopCh); err != nil {
			d.logger.Error("Failed to restart clipboard monitoring", zap.Error(err))
		} else {
			d.logger.Info("Successfully triggered clipboard monitoring restart",
				zap.Int("restart_count", d.monitoringRestartCount))
		}
	} else {
		// Monitoring is healthy
		if d.monitoringRestartCount > 0 {
			d.logger.Info("Clipboard monitoring is healthy",
				zap.String("mode", status.Mode),
				zap.Duration("time_since_activity", now.Sub(status.LastActivity)))
		}
	}

	d.lastMonitoringCheck = now
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
	return common.NewDaemonLogger(cfg)
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
	case "history.delete":
		return d.handleHistoryDeleteRequest(req)
	case "history.show":
		return d.handleHistoryShowRequest(req)
	case "history.stats":
		return d.handleHistoryStatsRequest(req)
	case "history.edit":
		return d.handleHistoryEditRequest(req)
	case "clip.get":
		return d.handleClipGetRequest(req)
	case "clip.set":
		return d.handleClipSetRequest(req)
	case "clip.watch":
		return d.handleClipWatchRequest(req)
	case "config.reload":
		return d.handleConfigReloadRequest(req)
	default:
		return &ipc.Response{
			Status:  "error",
			Message: fmt.Sprintf("Unknown command: %s", req.Command),
		}
	}
}

// handleHistoryShowRequest handles showing specific history entries by IDs or hashes
func (d *Daemon) handleHistoryShowRequest(req *ipc.Request) *ipc.Response {
	d.logger.Debug("Processing history show request", zap.Any("args", req.Args))

	// Check for single ID argument
	if singleID, ok := req.Args["id"]; ok {
		return d.handleShowByIDs([]interface{}{singleID})
	}

	d.logger.Warn("History show request missing valid arguments")
	return &ipc.Response{
		Status:  "error",
		Message: "Missing required arguments. Use 'ids', 'hashes', 'id', or 'hash'",
	}
}

// handleShowByIDs handles showing content by IDs
func (d *Daemon) handleShowByIDs(rawIDs interface{}) *ipc.Response {
	// Parse IDs from various possible formats
	var ids []int64

	switch v := rawIDs.(type) {
	case []interface{}:
		// Array of mixed types (most common from JSON)
		for i, rawID := range v {
			id, err := d.parseID(rawID, i)
			if err != nil {
				return &ipc.Response{
					Status:  "error",
					Message: err.Error(),
				}
			}
			ids = append(ids, id)
		}
	case []int64:
		// Already in correct format
		ids = v
	case []int:
		// Convert int to int64
		for _, id := range v {
			ids = append(ids, int64(id))
		}
	case []float64:
		// Convert float64 to int64 (common from JSON)
		for i, id := range v {
			if id != float64(int64(id)) {
				return &ipc.Response{
					Status:  "error",
					Message: fmt.Sprintf("ID at index %d has fractional part: %v", i, id),
				}
			}
			ids = append(ids, int64(id))
		}
	default:
		return &ipc.Response{
			Status:  "error",
			Message: fmt.Sprintf("Invalid type for 'ids' argument: expected array, got %T", rawIDs),
		}
	}

	if len(ids) == 0 {
		d.logger.Debug("No valid IDs provided in request")
		return &ipc.Response{
			Status: "ok",
			Data:   []*types.ClipboardContent{},
		}
	}

	// Use Query method with ID filter for better abstraction
	contents, err := d.storage.Query(storage.QueryOptions{
		IDs: ids,
	})
	if err != nil {
		d.logger.Error("Failed to get contents by IDs", zap.Int64s("ids", ids), zap.Error(err))
		return &ipc.Response{
			Status:  "error",
			Message: fmt.Sprintf("Failed to retrieve contents: %v", err),
		}
	}

	d.logger.Debug("Successfully retrieved contents by IDs using Query", 
		zap.Int("count", len(contents)), 
		zap.Int64s("requested_ids", ids))

	return &ipc.Response{
		Status: "ok",
		Data:   contents,
	}
}

// handleShowByHashes handles showing content by hashes
func (d *Daemon) handleShowByHashes(rawHashes interface{}) *ipc.Response {
	// Parse hashes from various possible formats
	var hashes []string

	switch v := rawHashes.(type) {
	case []interface{}:
		// Array of mixed types (most common from JSON)
		for i, rawHash := range v {
			if hash, ok := rawHash.(string); ok && hash != "" {
				hashes = append(hashes, hash)
			} else {
				return &ipc.Response{
					Status:  "error",
					Message: fmt.Sprintf("Invalid hash at index %d: expected non-empty string, got %T", i, rawHash),
				}
			}
		}
	case []string:
		// Already in correct format
		hashes = v
	default:
		return &ipc.Response{
			Status:  "error",
			Message: fmt.Sprintf("Invalid type for 'hashes' argument: expected array, got %T", rawHashes),
		}
	}

	if len(hashes) == 0 {
		d.logger.Debug("No valid hashes provided in request")
		return &ipc.Response{
			Status: "ok",
			Data:   []*types.ClipboardContent{},
		}
	}

	// Use Query method with hash filter for better abstraction
	contents, err := d.storage.Query(storage.QueryOptions{
		Hashes: hashes,
	})
	if err != nil {
		d.logger.Error("Failed to get contents by hashes", zap.Strings("hashes", hashes), zap.Error(err))
		return &ipc.Response{
			Status:  "error",
			Message: fmt.Sprintf("Failed to retrieve contents: %v", err),
		}
	}

	d.logger.Debug("Successfully retrieved contents by hashes using Query", 
		zap.Int("count", len(contents)), 
		zap.Strings("requested_hashes", hashes))

	return &ipc.Response{
		Status: "ok",
		Data:   contents,
	}
}

// parseID parses a single ID from various possible types
func (d *Daemon) parseID(rawID interface{}, index int) (int64, error) {
	switch v := rawID.(type) {
	case float64:
		// Most common for JSON numbers
		if v != float64(int64(v)) {
			return 0, fmt.Errorf("ID at index %d has fractional part: %v", index, v)
		}
		id := int64(v)
		if id <= 0 {
			return 0, fmt.Errorf("ID at index %d must be positive, got %d", index, id)
		}
		return id, nil
	case int:
		id := int64(v)
		if id <= 0 {
			return 0, fmt.Errorf("ID at index %d must be positive, got %d", index, id)
		}
		return id, nil
	case int64:
		if v <= 0 {
			return 0, fmt.Errorf("ID at index %d must be positive, got %d", index, v)
		}
		return v, nil
	case string:
		// Allow string IDs like "123"
		id, err := strconv.ParseInt(v, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("ID at index %d is not a valid integer: %s", index, v)
		}
		if id <= 0 {
			return 0, fmt.Errorf("ID at index %d must be positive, got %d", index, id)
		}
		return id, nil
	default:
		return 0, fmt.Errorf("ID at index %d has unsupported type: %T (value: %v)", index, v, v)
	}
}

// handleHistoryListRequest handles history list requests using daemon's storage
func (d *Daemon) handleHistoryListRequest(req *ipc.Request) *ipc.Response {
	d.logger.Debug("Processing history list request")

	// Check for hash or hashes argument
	var hashes []string
	if h, ok := req.Args["hash"].(string); ok && h != "" {
		hashes = append(hashes, h)
	}
	if hs, ok := req.Args["hashes"].([]interface{}); ok {
		for _, hash := range hs {
			if hashStr, ok := hash.(string); ok {
				hashes = append(hashes, hashStr)
			}
		}
	}

	if len(hashes) > 0 {
		// Use Query method with hash filter for better abstraction
		contents, err := d.storage.Query(storage.QueryOptions{
			Hashes: hashes,
		})
		if err != nil {
			d.logger.Error("Failed to get contents by hashes using Query", zap.Error(err))
			return &ipc.Response{
				Status:  "error",
				Message: fmt.Sprintf("Failed to get content(s): %v", err),
			}
		}
		return &ipc.Response{
			Status: "ok",
			Data:   contents,
		}
	}

	// Parse request arguments for normal list
	limit := int64(10) // default
	if l, ok := req.Args["limit"].(float64); ok {
		limit = int64(l)
	}

	reverse := false
	if r, ok := req.Args["reverse"].(bool); ok {
		reverse = r
	}

	contentType := types.ContentType("")
	if t, ok := req.Args["type"].(string); ok {
		contentType = types.ContentType(t)
	}

	// Parse tags filter
	var tagsFilter []string
	if tagsRaw, hasTags := req.Args["tags"]; hasTags {
		switch v := tagsRaw.(type) {
		case []interface{}:
			for _, tag := range v {
				if tagStr, ok := tag.(string); ok {
					tagsFilter = append(tagsFilter, tagStr)
				}
			}
		case []string:
			tagsFilter = v
		}
	}

	// Build query options
	options := storage.QueryOptions{
		Limit:       limit,
		ContentType: contentType,
		Tags:        tagsFilter,
		SortBy:      storage.SortByCreated,
	}
	
	// Set sort order based on reverse flag
	if reverse {
		options.SortOrder = storage.SortAsc
	} else {
		options.SortOrder = storage.SortDesc
	}

	// Parse time-based filters
	if since, ok := req.Args["since"].(string); ok {
		if sinceTime, err := time.Parse(time.RFC3339, since); err == nil {
			options.After = sinceTime
		}
	}

	if before, ok := req.Args["before"].(string); ok {
		if beforeTime, err := time.Parse(time.RFC3339, before); err == nil {
			options.Before = beforeTime
		}
	}

	// Use storage Query method
	contents, err := d.storage.Query(options)
	if err != nil {
		d.logger.Error("Failed to query history from storage", zap.Error(err))
		return &ipc.Response{
			Status:  "error",
			Message: fmt.Sprintf("Failed to get history: %v", err),
		}
	}

	return &ipc.Response{
		Status: "ok",
		Data:   contents,
	}
}

// handleHistoryDeleteRequest handles deleting history entries
func (d *Daemon) handleHistoryDeleteRequest(req *ipc.Request) *ipc.Response {
	d.logger.Info("=== DELETE REQUEST STARTED ===", zap.Any("raw_args", req.Args))

	// Parse request arguments with detailed logging
	d.logger.Info("=== PARSING ARGUMENTS ===")
	for key, value := range req.Args {
		d.logger.Info("Arg", zap.String("key", key), zap.Any("value", value), zap.String("type", fmt.Sprintf("%T", value)))
	}

	var hashes []string
	if h, ok := req.Args["hashes"].([]interface{}); ok {
		d.logger.Info("Found hashes argument", zap.Int("count", len(h)))
		for _, hash := range h {
			if hashStr, ok := hash.(string); ok {
				hashes = append(hashes, hashStr)
				d.logger.Info("Parsed hash", zap.String("hash", hashStr))
			} else {
				d.logger.Warn("Invalid hash type", zap.Any("hash", hash), zap.String("type", fmt.Sprintf("%T", hash)))
			}
		}
	} else {
		d.logger.Info("No hashes argument found or wrong type")
	}

	var ids []int64
	if i, ok := req.Args["ids"].([]interface{}); ok {
		d.logger.Info("Found ids argument", zap.Int("count", len(i)))
		for _, id := range i {
			if idFloat, ok := id.(float64); ok {
				idInt := int64(idFloat)
				ids = append(ids, idInt)
				d.logger.Info("Parsed ID", zap.Float64("original", idFloat), zap.Int64("converted", idInt))
			} else {
				d.logger.Warn("Invalid ID type", zap.Any("id", id), zap.String("type", fmt.Sprintf("%T", id)))
			}
		}
	} else {
		d.logger.Info("No ids argument found or wrong type")
	}

	all := false
	if a, ok := req.Args["all"].(bool); ok {
		all = a
	}

	olderThan := time.Time{}
	if older, ok := req.Args["older_than"].(string); ok {
		if olderTime, err := time.Parse(time.RFC3339, older); err == nil {
			olderThan = olderTime
		}
	}

	typeFilter := types.ContentType("")
	if t, ok := req.Args["type"].(string); ok {
		typeFilter = types.ContentType(t)
	}

	d.logger.Info("History delete request",
		zap.Strings("hashes", hashes),
		zap.Int64s("ids", ids),
		zap.Bool("all", all),
		zap.Time("older_than", olderThan),
		zap.String("type", string(typeFilter)))

	var deletedCount int
	var err error

	// Handle different deletion scenarios using unified Delete method
	if all {
		// Delete all content - still use explicit method for safety
		err = d.storage.DeleteAllContent()
		if err == nil {
			// Get count before deletion for response
			deletedCount, _ = d.storage.CountContent()
		}
	} else if len(hashes) > 0 || len(ids) > 0 || !olderThan.IsZero() || typeFilter != "" {
		// Use unified query-based deletion for all other cases
		queryOptions := storage.QueryOptions{}
		
		// Add filters based on provided criteria
		if len(hashes) > 0 {
			queryOptions.Hashes = hashes
			d.logger.Info("Added hashes to query options", zap.Strings("hashes", hashes))
		}
		if len(ids) > 0 {
			queryOptions.IDs = ids
			d.logger.Info("Added IDs to query options", zap.Int64s("ids", ids))
		}
		if !olderThan.IsZero() {
			queryOptions.Before = olderThan
			d.logger.Info("Added Before timestamp to query options", zap.Time("before", olderThan))
		}
		if typeFilter != "" {
			queryOptions.ContentType = typeFilter
			d.logger.Info("Added content type filter to query options", zap.String("type", string(typeFilter)))
		}
		
		d.logger.Info("=== CALLING STORAGE.DELETE ===", zap.Any("query_options", queryOptions))
		
		// Execute unified deletion
		deletedCount, err = d.storage.Delete(queryOptions)
		
		d.logger.Info("=== STORAGE.DELETE RESULT ===", zap.Int("deleted_count", deletedCount), zap.Error(err))
	} else {
		return &ipc.Response{
			Status:  "error",
			Message: "No valid deletion criteria provided",
		}
	}

	if err != nil {
		d.logger.Error("Failed to delete contents", zap.Error(err))
		return &ipc.Response{
			Status:  "error",
			Message: fmt.Sprintf("Failed to delete contents: %v", err),
		}
	}

	d.logger.Info("Successfully deleted history entries",
		zap.Int("deleted_count", deletedCount),
		zap.Bool("all", all),
		zap.Strings("deleted_hashes", hashes),
		zap.Int64s("deleted_ids", ids))

	return &ipc.Response{
		Status: "ok",
		Data:   deletedCount,
	}
}

// handleHistoryStatsRequest handles history statistics request
func (d *Daemon) handleHistoryStatsRequest(req *ipc.Request) *ipc.Response {
	d.logger.Debug("Processing history stats request")

	// Use the comprehensive GetStats method from our storage implementation
	storageStats, err := d.storage.GetStats()
	if err != nil {
		d.logger.Error("Failed to get storage statistics", zap.Error(err))
		return &ipc.Response{
			Status:  "error",
			Message: fmt.Sprintf("Failed to get statistics: %v", err),
		}
	}

	// Convert to the format expected by the CLI
	stats := map[string]interface{}{
		"total_entries":    storageStats.TotalItems,
		"total_size":       storageStats.TotalSize,
		"average_size":     storageStats.AverageSize,
		"compressed_items": storageStats.CompressedItems,
		"compression_ratio": storageStats.CompressionRatio,
		"type_counts":      make(map[string]interface{}),
		"device_counts":    make(map[string]interface{}),
		"oldest_entry":     nil,
		"newest_entry":     nil,
	}

	// Convert type stats
	for contentType, typeStat := range storageStats.ByType {
		stats["type_counts"].(map[string]interface{})[string(contentType)] = map[string]interface{}{
			"count":        typeStat.Count,
			"total_size":   typeStat.TotalSize,
			"average_size": typeStat.AverageSize,
			"min_size":     typeStat.MinSize,
			"max_size":     typeStat.MaxSize,
		}
	}

	// Convert device stats
	for deviceID, deviceStat := range storageStats.ByDevice {
		stats["device_counts"].(map[string]interface{})[deviceID] = map[string]interface{}{
			"count":      deviceStat.Count,
			"total_size": deviceStat.TotalSize,
			"first_seen": deviceStat.FirstSeen.Format(time.RFC3339),
			"last_seen":  deviceStat.LastSeen.Format(time.RFC3339),
		}
	}

	// Add oldest/newest entries
	if !storageStats.OldestItem.IsZero() {
		stats["oldest_entry"] = storageStats.OldestItem.Format(time.RFC3339)
	}
	if !storageStats.NewestItem.IsZero() {
		stats["newest_entry"] = storageStats.NewestItem.Format(time.RFC3339)
	}

	// Add top occurrences if available
	if len(storageStats.TopOccurrences) > 0 {
		topOccurrences := make([]interface{}, len(storageStats.TopOccurrences))
		for i, occ := range storageStats.TopOccurrences {
			topOccurrences[i] = map[string]interface{}{
				"hash":              occ.Hash,
				"total_occurrences": occ.TotalOccurrences,
				"content_type":      string(occ.ContentType),
				"first_seen":        occ.FirstSeen.Format(time.RFC3339),
				"last_seen":         occ.LastSeen.Format(time.RFC3339),
			}
			if occ.AverageFrequency > 0 {
				topOccurrences[i].(map[string]interface{})["average_frequency_seconds"] = occ.AverageFrequency.Seconds()
			}
		}
		stats["top_occurrences"] = topOccurrences
	}

	d.logger.Info("Generated comprehensive storage statistics",
		zap.Int64("total_entries", storageStats.TotalItems),
		zap.Int64("total_size", storageStats.TotalSize),
		zap.Float64("average_size", storageStats.AverageSize),
		zap.Int("content_types", len(storageStats.ByType)),
		zap.Int("devices", len(storageStats.ByDevice)))

	return &ipc.Response{
		Status: "ok",
		Data:   stats,
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
		Tags: []string{}, // Initialize tags as empty slice
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

// handleConfigReloadRequest handles config reload requests
func (d *Daemon) handleConfigReloadRequest(req *ipc.Request) *ipc.Response {
	d.logger.Info("Received configuration reload request")

	// Load new configuration
	newCfg, err := config.Load("")
	if err != nil {
		d.logger.Error("Failed to load new configuration", zap.Error(err))
		return &ipc.Response{
			Status:  "error",
			Message: fmt.Sprintf("Failed to load configuration: %v", err),
		}
	}

	d.logger.Info("Successfully loaded new configuration")

	// Check what needs to be reloaded/restarted
	var changes []string
	var restartSync bool
	var restartClipboard bool

	// Compare sync configuration
	if d.cfg.Sync.Enabled != newCfg.Sync.Enabled {
		changes = append(changes, "sync enabled status")
		restartSync = true
	}
	if d.cfg.Sync.DeviceName != newCfg.Sync.DeviceName {
		changes = append(changes, "device name")
		restartSync = true
	}
	if d.cfg.Sync.PairingEnabled != newCfg.Sync.PairingEnabled {
		changes = append(changes, "pairing enabled status")
		restartSync = true
	}
	if d.cfg.Sync.DiscoveryMethod != newCfg.Sync.DiscoveryMethod {
		changes = append(changes, "discovery method")
		restartSync = true
	}
	if d.cfg.Sync.ListenPort != newCfg.Sync.ListenPort {
		changes = append(changes, "listen port")
		restartSync = true
	}

	// Compare clipboard configuration
	if d.cfg.StealthMode != newCfg.StealthMode {
		changes = append(changes, "stealth mode")
		restartClipboard = true
	}
	if d.cfg.PollingInterval != newCfg.PollingInterval {
		changes = append(changes, "polling interval")
		restartClipboard = true
	}
	if d.cfg.HTML.ExtractText != newCfg.HTML.ExtractText {
		changes = append(changes, "HTML text extraction")
		restartClipboard = true
	}

	// Compare logging configuration
	if d.cfg.Log.Level != newCfg.Log.Level {
		changes = append(changes, "log level")
	}

	if len(changes) == 0 {
		d.logger.Info("No configuration changes detected")
		return &ipc.Response{
			Status:  "ok",
			Message: "Configuration reloaded - no changes detected",
		}
	}

	d.logger.Info("Configuration changes detected", zap.Strings("changes", changes))

	// Apply the new configuration
	d.cfg = newCfg

	// Restart sync component if needed
	if restartSync {
		d.logger.Info("Restarting sync component due to configuration changes")
		
		// Stop existing sync
		if d.sync != nil {
			if err := d.sync.Stop(); err != nil {
				d.logger.Warn("Failed to stop sync component", zap.Error(err))
			}
			d.sync = nil
		}

		// Start new sync if enabled
		if d.cfg.Sync.Enabled {
			syncNode, err := p2p.NewNode(d.ctx, d.cfg, d.logger)
			if err != nil {
				d.logger.Error("Failed to create new sync component", zap.Error(err))
				return &ipc.Response{
					Status:  "error",
					Message: fmt.Sprintf("Failed to restart sync: %v", err),
				}
			}
			d.sync = syncNode

			if err := d.sync.Start(); err != nil {
				d.logger.Error("Failed to start new sync component", zap.Error(err))
				return &ipc.Response{
					Status:  "error",
					Message: fmt.Sprintf("Failed to start sync: %v", err),
				}
			}
			d.logger.Info("Successfully restarted sync component")
		} else {
			d.logger.Info("Sync disabled in new configuration")
		}
	}

	// Restart clipboard monitoring if needed
	if restartClipboard {
		d.logger.Info("Restarting clipboard monitoring due to configuration changes")
		
		// Create new clipboard instance
		newClipboard := clipboard.NewClipboardWithFullConfig(d.logger, d.cfg)
		if newClipboard == nil {
			d.logger.Error("Failed to create new clipboard instance")
			return &ipc.Response{
				Status:  "error",
				Message: "Failed to create new clipboard instance",
			}
		}

		// Stop old clipboard monitoring
		if d.clipboard != nil {
			d.clipboard.Close()
		}

		// Update to new clipboard
		d.clipboard = newClipboard

		// Restart monitoring with existing channels
		go d.clipboard.MonitorChanges(d.contentCh, d.stopCh)
		d.logger.Info("Successfully restarted clipboard monitoring")
	}

	message := fmt.Sprintf("Configuration reloaded successfully. Changes applied: %s", strings.Join(changes, ", "))
	d.logger.Info("Configuration reload completed", zap.String("summary", message))

	return &ipc.Response{
		Status:  "ok",
		Message: message,
	}
}

// handleHistoryEditRequest handles editing history entries (e.g., tags)
func (d *Daemon) handleHistoryEditRequest(req *ipc.Request) *ipc.Response {
	d.logger.Debug("Processing history edit request", zap.Any("args", req.Args))

	// Parse ID from request
	idRaw, hasID := req.Args["id"]
	if !hasID {
		return &ipc.Response{
			Status:  "error",
			Message: "Missing required 'id' parameter",
		}
	}

	id, err := d.parseID(idRaw, 0)
	if err != nil {
		return &ipc.Response{
			Status:  "error",
			Message: fmt.Sprintf("Invalid ID: %v", err),
		}
	}

	// Parse operation type
	operation, hasOp := req.Args["operation"].(string)
	if !hasOp {
		return &ipc.Response{
			Status:  "error",
			Message: "Missing required 'operation' parameter",
		}
	}

	// Parse tags based on operation
	var tags []string
	if tagsRaw, hasTags := req.Args["tags"]; hasTags {
		switch v := tagsRaw.(type) {
		case []interface{}:
			for _, tag := range v {
				if tagStr, ok := tag.(string); ok {
					tags = append(tags, tagStr)
				}
			}
		case []string:
			tags = v
		case string:
			tags = []string{v}
		default:
			return &ipc.Response{
				Status:  "error",
				Message: "Invalid 'tags' parameter format",
			}
		}
	}

	d.logger.Info("Edit request details",
		zap.Int64("id", id),
		zap.String("operation", operation),
		zap.Strings("tags", tags))

	// Get existing content to verify it exists and get current tags
	contents, err := d.storage.Query(storage.QueryOptions{
		IDs: []int64{id},
	})
	if err != nil {
		d.logger.Error("Failed to get content for edit", zap.Int64("id", id), zap.Error(err))
		return &ipc.Response{
			Status:  "error",
			Message: fmt.Sprintf("Failed to get content: %v", err),
		}
	}

	if len(contents) == 0 {
		return &ipc.Response{
			Status:  "error",
			Message: fmt.Sprintf("No content found with ID %d", id),
		}
	}

	content := contents[0]
	currentTags := make([]string, len(content.Tags))
	copy(currentTags, content.Tags)

	// Apply the operation
	switch operation {
	case "add":
		// Add new tags, avoiding duplicates
		for _, newTag := range tags {
			found := false
			for _, existing := range currentTags {
				if existing == newTag {
					found = true
					break
				}
			}
			if !found {
				currentTags = append(currentTags, newTag)
			}
		}
	case "remove":
		// Remove specified tags
		var filtered []string
		for _, existing := range currentTags {
			shouldKeep := true
			for _, toRemove := range tags {
				if existing == toRemove {
					shouldKeep = false
					break
				}
			}
			if shouldKeep {
				filtered = append(filtered, existing)
			}
		}
		currentTags = filtered
	case "set":
		// Replace all tags with the new ones
		currentTags = tags
	case "clear":
		// Remove all tags
		currentTags = []string{}
	default:
		return &ipc.Response{
			Status:  "error",
			Message: fmt.Sprintf("Unknown operation: %s. Valid operations are: add, remove, set, clear", operation),
		}
	}

	// Update the content with new tags
	updates := map[string]interface{}{
		"tags": currentTags,
	}

	err = d.storage.UpdateContent(id, updates)
	if err != nil {
		d.logger.Error("Failed to update content", zap.Int64("id", id), zap.Error(err))
		return &ipc.Response{
			Status:  "error",
			Message: fmt.Sprintf("Failed to update content: %v", err),
		}
	}

	d.logger.Info("Successfully updated content tags",
		zap.Int64("id", id),
		zap.String("operation", operation),
		zap.Strings("new_tags", currentTags))

	// Get updated content to return
	updatedContents, err := d.storage.Query(storage.QueryOptions{
		IDs: []int64{id},
	})
	if err != nil || len(updatedContents) == 0 {
		// Still return success even if we can't get the updated content
		return &ipc.Response{
			Status:  "ok",
			Message: fmt.Sprintf("Successfully updated tags for content ID %d", id),
		}
	}

	// Return the updated content
	return &ipc.Response{
		Status:  "ok",
		Data:    updatedContents[0],
		Message: fmt.Sprintf("Successfully updated tags for content ID %d", id),
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
