package daemon

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

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
	storage   *storage.BoltStorage
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

	// Initialize storage
	d.logger.Info("📦 Initializing storage...")
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
	d.logger.Info("✅ Storage initialized successfully")

	// Initialize clipboard
	d.logger.Info("📋 Initializing clipboard...")
	clipboard := clipboard.NewClipboardWithConfig(d.logger, d.cfg.StealthMode)
	d.logger.Info("📋 Clipboard NewClipboard() called", 
		zap.Bool("is_nil", clipboard == nil),
		zap.String("clipboard_type", fmt.Sprintf("%T", clipboard)),
		zap.Bool("stealth_mode", d.cfg.StealthMode))
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
				if err := d.storage.SaveContent(content); err != nil {
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

	// Debug log each content entry
	for i, content := range contents {
		d.logger.Debug("History entry details",
			zap.Int("index", i),
			zap.String("type", string(content.Type)),
			zap.Int("data_size", len(content.Data)),
			zap.String("hash", content.Hash),
			zap.Time("created", content.Created),
			zap.Bool("has_data", content.Data != nil),
			zap.Bool("has_type", content.Type != ""),
			zap.Bool("has_hash", content.Hash != ""),
			zap.Bool("has_created", !content.Created.IsZero()))
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
	d.logger.Debug("Processing history show request", zap.Any("args", req.Args))

	// Parse request arguments
	hash := ""
	if h, ok := req.Args["hash"].(string); ok {
		hash = h
	}

	if hash == "" {
		return &ipc.Response{
			Status:  "error",
			Message: "No hash provided",
		}
	}

	d.logger.Info("History show request", zap.String("hash", hash))

	// Get all content to find the specific hash
	allContents, err := d.storage.GetAllContents()
	if err != nil {
		d.logger.Error("Failed to get all contents for show", zap.Error(err))
		return &ipc.Response{
			Status:  "error",
			Message: fmt.Sprintf("Failed to get contents: %v", err),
		}
	}

	// Find the content with the specified hash
	var foundContent *types.ClipboardContent
	for _, content := range allContents {
		if content.Hash == hash {
			foundContent = content
			break
		}
	}

	if foundContent == nil {
		d.logger.Warn("Content not found", zap.String("hash", hash))
		return &ipc.Response{
			Status:  "error",
			Message: fmt.Sprintf("Content with hash %s not found", hash),
		}
	}

	d.logger.Info("Found content for show", 
		zap.String("hash", foundContent.Hash),
		zap.String("type", string(foundContent.Type)),
		zap.Int("size", len(foundContent.Data)),
		zap.Time("created", foundContent.Created))

	return &ipc.Response{
		Status: "ok",
		Data:   foundContent,
	}
}

// handleHistoryDeleteRequest handles deleting history entries
func (d *Daemon) handleHistoryDeleteRequest(req *ipc.Request) *ipc.Response {
	d.logger.Debug("Processing history delete request", zap.Any("args", req.Args))

	// Parse request arguments
	var hashes []string
	if h, ok := req.Args["hashes"].([]interface{}); ok {
		for _, hash := range h {
			if hashStr, ok := hash.(string); ok {
				hashes = append(hashes, hashStr)
			}
		}
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

	typeFilter := ""
	if t, ok := req.Args["type"].(string); ok {
		typeFilter = t
	}

	d.logger.Info("History delete request", 
		zap.Strings("hashes", hashes),
		zap.Bool("all", all),
		zap.Time("older_than", olderThan),
		zap.String("type", typeFilter))

	// Get all content to filter
	allContents, err := d.storage.GetAllContents()
	if err != nil {
		d.logger.Error("Failed to get all contents for deletion", zap.Error(err))
		return &ipc.Response{
			Status:  "error",
			Message: fmt.Sprintf("Failed to get contents: %v", err),
		}
	}

	// Filter contents to delete
	var contentsToDelete []*types.ClipboardContent
	for _, content := range allContents {
		shouldDelete := false

		// Check if this content should be deleted based on criteria
		if all {
			shouldDelete = true
		} else if len(hashes) > 0 {
			// Check if this content's hash matches any of the requested hashes
			for _, hash := range hashes {
				if content.Hash == hash {
					shouldDelete = true
					break
				}
			}
		} else if !olderThan.IsZero() {
			// Check if content is older than the specified time
			if content.Created.Before(olderThan) {
				shouldDelete = true
			}
		} else if typeFilter != "" {
			// Check if content type matches the filter
			if string(content.Type) == typeFilter {
				shouldDelete = true
			}
		}

		if shouldDelete {
			contentsToDelete = append(contentsToDelete, content)
		}
	}

	if len(contentsToDelete) == 0 {
		d.logger.Info("No content found matching deletion criteria")
		return &ipc.Response{
			Status: "ok",
			Data:   0,
		}
	}

	// Delete the filtered contents
	err = d.storage.DeleteContents(contentsToDelete)
	if err != nil {
		d.logger.Error("Failed to delete contents", zap.Error(err))
		return &ipc.Response{
			Status:  "error",
			Message: fmt.Sprintf("Failed to delete contents: %v", err),
		}
	}

	d.logger.Info("Successfully deleted history entries", 
		zap.Int("deleted_count", len(contentsToDelete)),
		zap.Bool("all", all),
		zap.Strings("deleted_hashes", func() []string {
			var hashes []string
			for _, content := range contentsToDelete {
				hashes = append(hashes, content.Hash)
			}
			return hashes
		}()))

	return &ipc.Response{
		Status: "ok",
		Data:   len(contentsToDelete),
	}
}

// handleHistoryStatsRequest handles history statistics request
func (d *Daemon) handleHistoryStatsRequest(req *ipc.Request) *ipc.Response {
	d.logger.Debug("Processing history stats request")

	// Get all content for statistics
	allContents, err := d.storage.GetAllContents()
	if err != nil {
		d.logger.Error("Failed to get all contents for stats", zap.Error(err))
		return &ipc.Response{
			Status:  "error",
			Message: fmt.Sprintf("Failed to get contents: %v", err),
		}
	}

	// Calculate statistics
	stats := map[string]interface{}{
		"total_entries": len(allContents),
		"total_size":    0,
		"type_counts":   make(map[string]int),
		"oldest_entry":  nil,
		"newest_entry":  nil,
	}

	var totalSize int64
	var oldestTime, newestTime time.Time
	var oldestContent, newestContent *types.ClipboardContent

	for _, content := range allContents {
		// Count total size
		totalSize += int64(len(content.Data))
		
		// Count by type
		typeStr := string(content.Type)
		stats["type_counts"].(map[string]int)[typeStr]++
		
		// Track oldest and newest
		if oldestTime.IsZero() || content.Created.Before(oldestTime) {
			oldestTime = content.Created
			oldestContent = content
		}
		if newestTime.IsZero() || content.Created.After(newestTime) {
			newestTime = content.Created
			newestContent = content
		}
	}

	stats["total_size"] = totalSize
	if oldestContent != nil {
		stats["oldest_entry"] = map[string]interface{}{
			"hash":    oldestContent.Hash,
			"type":    string(oldestContent.Type),
			"created": oldestContent.Created.Format(time.RFC3339),
			"size":    len(oldestContent.Data),
		}
	}
	if newestContent != nil {
		stats["newest_entry"] = map[string]interface{}{
			"hash":    newestContent.Hash,
			"type":    string(newestContent.Type),
			"created": newestContent.Created.Format(time.RFC3339),
			"size":    len(newestContent.Data),
		}
	}

	d.logger.Info("Generated history statistics", 
		zap.Int("total_entries", len(allContents)),
		zap.Int64("total_size", totalSize),
		zap.Any("type_counts", stats["type_counts"]))

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