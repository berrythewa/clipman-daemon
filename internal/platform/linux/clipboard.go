//go:build linux
// +build linux

package platform

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	// "syscall"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"go.uber.org/zap"
)

// ClipboardBackend represents different clipboard access methods
type ClipboardBackend int

const (
	BackendXClip ClipboardBackend = iota
	BackendXSel
	BackendWLClipboard
	BackendDisabled
)

// MonitoringMode represents different monitoring strategies
type MonitoringMode int

const (
	MonitorModeXFixes MonitoringMode = iota // X11 XFixes extension (event-based)
	MonitorModeWayland                      // Wayland wl-paste -w (event-based)
	MonitorModePolling                      // Adaptive polling (fallback)
)

// LinuxClipboard provides a Linux clipboard abstraction using CLI tools
type LinuxClipboard struct {
	mu             sync.Mutex
	lastContent    string
	lastContentHash string
	isRunning      bool
	logger         *zap.Logger
	ctx            context.Context
	cancel         context.CancelFunc
	backend        ClipboardBackend
	monitorMode    MonitoringMode
	
	// Smart polling parameters
	inactiveStreak int
	baseInterval   time.Duration
	maxInterval    time.Duration
	
	// Change detection optimization
	lastFormats   []string
	skipCounter   int
	skipThreshold int
	
	// Process management for event-based monitoring
	monitorProc   *os.Process
}

// NewClipboard creates a new LinuxClipboard instance
func NewClipboard(logger *zap.Logger) *LinuxClipboard {
	if logger == nil {
		logger = zap.NewNop()
	}

	ctx, cancel := context.WithCancel(context.Background())
	clipboard := &LinuxClipboard{
		logger:        logger,
		ctx:           ctx,
		cancel:        cancel,
		baseInterval:  100 * time.Millisecond,  // 100ms base - much more responsive
		maxInterval:   500 * time.Millisecond,  // 500ms max - prevent missing changes
		skipThreshold: 1, // Skip content checks less often to catch more changes
	}
	
	// Detect and setup the best available backend and monitoring mode
	clipboard.detectBackend()
	clipboard.detectMonitoringMode()
	return clipboard
}

// detectBackend determines the best clipboard backend for the current environment
func (c *LinuxClipboard) detectBackend() {
	// Check for Wayland first
	if os.Getenv("WAYLAND_DISPLAY") != "" {
		if c.checkCommand("wl-copy") && c.checkCommand("wl-paste") {
			c.backend = BackendWLClipboard
			c.logger.Info("Using Wayland clipboard backend (wl-clipboard)")
			return
		}
		c.logger.Warn("Wayland detected but wl-clipboard not available")
	}
	
	// Check for X11 tools
	if c.checkCommand("xclip") {
		c.backend = BackendXClip
		c.logger.Info("Using X11 clipboard backend (xclip)")
		return
	}
	
	if c.checkCommand("xsel") {
		c.backend = BackendXSel  
		c.logger.Info("Using X11 clipboard backend (xsel)")
		return
	}
	
	// No clipboard tools available
	c.backend = BackendDisabled
	c.logger.Warn("No clipboard tools available (xclip, xsel, wl-clipboard). Clipboard access disabled.")
}

// detectMonitoringMode determines the best monitoring strategy
func (c *LinuxClipboard) detectMonitoringMode() {
	c.logger.Info("Detecting optimal clipboard monitoring method")
	
	// Check for X11 with XFixes extension (most efficient)
	if c.isX11Session() && c.checkXFixesSupport() {
		c.monitorMode = MonitorModeXFixes
		c.logger.Info("Using X11 XFixes event-based monitoring (most efficient)")
		return
	}
	
	// Check for Wayland with wl-paste -w support
	if c.isWaylandSession() && c.checkCommand("wl-paste") {
		c.monitorMode = MonitorModeWayland
		c.logger.Info("Using Wayland event-based monitoring (wl-paste -w)")
		return
	}
	
	// Fallback to adaptive polling
	c.monitorMode = MonitorModePolling
	c.logger.Info("Using adaptive polling monitoring (fallback)")
}

// checkCommand checks if a command is available in PATH
func (c *LinuxClipboard) checkCommand(command string) bool {
	_, err := exec.LookPath(command)
	return err == nil
}

// isX11Session checks if running in X11 environment
func (c *LinuxClipboard) isX11Session() bool {
	return os.Getenv("DISPLAY") != ""
}

// isWaylandSession checks if running in Wayland environment
func (c *LinuxClipboard) isWaylandSession() bool {
	return os.Getenv("WAYLAND_DISPLAY") != ""
}

// checkXFixesSupport checks if X11 XFixes extension is available
func (c *LinuxClipboard) checkXFixesSupport() bool {
	if !c.checkCommand("xprop") {
		return false
	}
	
	// Try to run xprop to verify X11 connectivity
	cmd := exec.Command("xprop", "-root", "-notype", "_NET_SUPPORTED")
	err := cmd.Run()
	return err == nil
}

// getAvailableFormats returns clipboard formats (less intrusive than reading content)
func (c *LinuxClipboard) getAvailableFormats() ([]string, error) {
	switch c.backend {
	case BackendXClip:
		if c.isX11Session() {
			cmd := exec.Command("xclip", "-selection", "clipboard", "-t", "TARGETS", "-o")
			output, err := cmd.Output()
			if err != nil {
				return []string{"text/plain"}, err // Fallback
			}
			return c.parseFormats(output), nil
		}
		
	case BackendWLClipboard:
		if c.isWaylandSession() {
			cmd := exec.Command("wl-paste", "--list-types")
			output, err := cmd.Output()
			if err != nil {
				return []string{"text/plain"}, err // Fallback
			}
			return c.parseFormats(output), nil
		}
	}
	
	return []string{"text/plain"}, nil
}

// parseFormats parses clipboard format output
func (c *LinuxClipboard) parseFormats(output []byte) []string {
	lines := strings.Split(string(output), "\n")
	var formats []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			formats = append(formats, line)
		}
	}
	return formats
}

// formatsEqual checks if two format slices are equal
func (c *LinuxClipboard) formatsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	aMap := make(map[string]bool)
	for _, v := range a {
		aMap[v] = true
	}
	for _, v := range b {
		if !aMap[v] {
			return false
		}
	}
	return true
}

// hashContent creates a simple hash for change detection
func (c *LinuxClipboard) hashContent(data string) string {
	if len(data) == 0 {
		return ""
	}
	
	var hash uint32
	for i, b := range []byte(data) {
		hash = (hash << 5) + hash + uint32(b)
		// Only hash first 4KB for performance
		if i > 4096 {
			break
		}
	}
	return fmt.Sprintf("%x", hash)
}

// Read returns the current clipboard content
func (c *LinuxClipboard) Read() (*types.ClipboardContent, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.backend == BackendDisabled {
		return nil, fmt.Errorf("clipboard access disabled - no tools available")
	}

	c.logger.Debug("Starting clipboard read operation", 
		zap.String("backend", c.backendName()))

	// Get available formats first to understand what's in clipboard
	formats, err := c.getAvailableFormats()
	if err != nil {
		c.logger.Debug("Failed to get clipboard formats during read", zap.Error(err))
	} else {
		c.logger.Debug("Available clipboard formats", zap.Strings("formats", formats))
	}

	// Check if clipboard contains files by looking at formats
	contentType := c.detectContentType(formats)
	c.logger.Debug("Detected content type", zap.String("type", string(contentType)))

	var content *types.ClipboardContent

	// Handle different content types
	switch contentType {
	case types.TypeFile:
		content, err = c.readClipboardFiles()
		if err != nil {
			c.logger.Debug("Failed to read files, falling back to text", zap.Error(err))
			// Fallback to text
			content, err = c.readClipboardText2()
		}
	default:
		content, err = c.readClipboardText2()
	}

	if err != nil {
		c.logger.Debug("Failed to read clipboard content", 
			zap.Error(err), 
			zap.String("attempted_type", string(contentType)))
		return nil, fmt.Errorf("failed to read clipboard: %w", err)
	}

	c.logger.Info("Successfully read clipboard content",
		zap.String("backend", c.backendName()),
		zap.String("detected_type", string(contentType)),
		zap.String("final_type", string(content.Type)),
		zap.Int("size", len(content.Data)),
		zap.Int("format_count", len(formats)))

	return content, nil
}

// detectContentType analyzes clipboard formats to determine content type
func (c *LinuxClipboard) detectContentType(formats []string) types.ContentType {
	c.logger.Debug("Analyzing clipboard formats for content type detection", 
		zap.Strings("formats", formats))

	// Check for file-related formats
	fileFormats := []string{
		"text/uri-list",
		"text/x-moz-url", 
		"application/x-kde-cutselection",
		"x-special/gnome-copied-files",
		"application/x-nautilus-desktop",
		"text/plain;charset=utf-8", // Sometimes files are also available as text
	}

	for _, format := range formats {
		for _, fileFormat := range fileFormats {
			if strings.Contains(strings.ToLower(format), strings.ToLower(fileFormat)) {
				c.logger.Debug("File format detected", 
					zap.String("format", format), 
					zap.String("matched", fileFormat))
				return types.TypeFile
			}
		}
	}

	c.logger.Debug("No file formats detected, defaulting to text")
	return types.TypeText
}

// readClipboardText2 reads text content (renamed to avoid conflict)
func (c *LinuxClipboard) readClipboardText2() (*types.ClipboardContent, error) {
	c.logger.Debug("Reading clipboard as text content")
	
	text, err := c.readClipboardText()
	if err != nil {
		return nil, err
	}

	content := &types.ClipboardContent{
		Type: types.TypeText,
		Data: []byte(text),
	}

	c.logger.Debug("Text content read successfully", 
		zap.Int("length", len(text)),
		zap.String("preview", c.getContentPreview(text, 100)))

	return content, nil
}

// readClipboardFiles reads file content from clipboard
func (c *LinuxClipboard) readClipboardFiles() (*types.ClipboardContent, error) {
	c.logger.Debug("Attempting to read clipboard as file content")

	var cmd *exec.Cmd
	var format string

	// Try different approaches to get file URIs
	switch c.backend {
	case BackendXClip:
		// Try uri-list format first
		c.logger.Debug("Trying xclip with text/uri-list format")
		cmd = exec.Command("xclip", "-selection", "clipboard", "-t", "text/uri-list", "-o")
		format = "text/uri-list"
	case BackendWLClipboard:
		// Try uri-list format first  
		c.logger.Debug("Trying wl-paste with text/uri-list format")
		cmd = exec.Command("wl-paste", "-t", "text/uri-list")
		format = "text/uri-list"
	default:
		c.logger.Debug("Backend doesn't support specific format reading, falling back to text")
		return c.readClipboardText2()
	}

	output, err := cmd.Output()
	if err != nil {
		c.logger.Debug("Failed to read with format, trying alternative", 
			zap.String("format", format), 
			zap.Error(err))
		
		// Try alternative formats
		return c.tryAlternativeFileFormats()
	}

	outputStr := strings.TrimSpace(string(output))
	if outputStr == "" {
		c.logger.Debug("Empty output from file format read, trying alternatives")
		return c.tryAlternativeFileFormats()
	}

	c.logger.Info("Successfully read file content from clipboard", 
		zap.String("format", format),
		zap.Int("length", len(outputStr)),
		zap.String("preview", c.getContentPreview(outputStr, 200)))

	// Parse file URIs
	files := c.parseFileURIs(outputStr)
	if len(files) == 0 {
		c.logger.Debug("No valid file URIs found, treating as text")
		return &types.ClipboardContent{
			Type: types.TypeText,
			Data: []byte(outputStr),
		}, nil
	}

	// Create file content
	content := &types.ClipboardContent{
		Type: types.TypeFile,
		Data: []byte(outputStr), // Store the raw URI list
	}

	c.logger.Info("File content processed successfully", 
		zap.Int("file_count", len(files)),
		zap.Strings("files", files))

	return content, nil
}

// tryAlternativeFileFormats tries different clipboard formats for files
func (c *LinuxClipboard) tryAlternativeFileFormats() (*types.ClipboardContent, error) {
	c.logger.Debug("Trying alternative file formats")

	alternativeFormats := []string{
		"x-special/gnome-copied-files",
		"application/x-kde-cutselection", 
		"text/x-moz-url",
	}

	for _, format := range alternativeFormats {
		c.logger.Debug("Trying alternative format", zap.String("format", format))
		
		var cmd *exec.Cmd
		switch c.backend {
		case BackendXClip:
			cmd = exec.Command("xclip", "-selection", "clipboard", "-t", format, "-o")
		case BackendWLClipboard:
			cmd = exec.Command("wl-paste", "-t", format)
		default:
			continue
		}

		output, err := cmd.Output()
		if err != nil {
			c.logger.Debug("Format failed", zap.String("format", format), zap.Error(err))
			continue
		}

		outputStr := strings.TrimSpace(string(output))
		if outputStr != "" {
			c.logger.Info("Successfully read with alternative format", 
				zap.String("format", format),
				zap.String("preview", c.getContentPreview(outputStr, 200)))
			
			return &types.ClipboardContent{
				Type: types.TypeFile,
				Data: []byte(outputStr),
			}, nil
		}
	}

	c.logger.Debug("All alternative formats failed, falling back to text")
	return c.readClipboardText2()
}

// parseFileURIs extracts file paths from URI list format
func (c *LinuxClipboard) parseFileURIs(uriList string) []string {
	var files []string
	lines := strings.Split(uriList, "\n")
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// Handle file:// URIs
		if strings.HasPrefix(line, "file://") {
			path := strings.TrimPrefix(line, "file://")
			// URL decode the path
			if decoded, err := exec.Command("printf", "%b", strings.ReplaceAll(path, "%", "\\x")).Output(); err == nil {
				path = string(decoded)
			}
			files = append(files, path)
			c.logger.Debug("Parsed file URI", zap.String("uri", line), zap.String("path", path))
		} else if strings.HasPrefix(line, "/") {
			// Direct file path
			files = append(files, line)
			c.logger.Debug("Found direct file path", zap.String("path", line))
		}
	}
	
	return files
}

// getContentPreview returns a safe preview of content for logging
func (c *LinuxClipboard) getContentPreview(content string, maxLen int) string {
	if len(content) <= maxLen {
		return content
	}
	return content[:maxLen] + "..."
}

// Write sets the clipboard content
func (c *LinuxClipboard) Write(content *types.ClipboardContent) error {
	if content == nil || len(content.Data) == 0 {
		return fmt.Errorf("no content to write")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.backend == BackendDisabled {
		return fmt.Errorf("clipboard access disabled - no tools available")
	}

	err := c.writeClipboardText(string(content.Data))
	if err != nil {
		return fmt.Errorf("failed to write clipboard: %w", err)
	}

	c.logger.Debug("Wrote clipboard content",
		zap.String("backend", c.backendName()),
		zap.String("type", string(content.Type)),
		zap.Int("size", len(content.Data)))

	return nil
}

// readClipboardText reads text from clipboard using the selected backend
func (c *LinuxClipboard) readClipboardText() (string, error) {
	var cmd *exec.Cmd
	
	switch c.backend {
	case BackendXClip:
		cmd = exec.Command("xclip", "-selection", "clipboard", "-o")
	case BackendXSel:
		cmd = exec.Command("xsel", "--clipboard", "--output")
	case BackendWLClipboard:
		cmd = exec.Command("wl-paste")
	default:
		return "", fmt.Errorf("no clipboard backend available")
	}

	output, err := cmd.Output()
	if err != nil {
		// Check for empty clipboard (common exit status 1)
		if strings.Contains(err.Error(), "exit status 1") {
			return "", nil // Empty clipboard is not an error
		}
		return "", err
	}
	
	return string(output), nil
}

// writeClipboardText writes text to clipboard using the selected backend
func (c *LinuxClipboard) writeClipboardText(text string) error {
	var cmd *exec.Cmd
	
	switch c.backend {
	case BackendXClip:
		cmd = exec.Command("xclip", "-selection", "clipboard", "-i")
	case BackendXSel:
		cmd = exec.Command("xsel", "--clipboard", "--input")
	case BackendWLClipboard:
		cmd = exec.Command("wl-copy")
	default:
		return fmt.Errorf("no clipboard backend available")
	}

	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

// backendName returns a human-readable name for the current backend
func (c *LinuxClipboard) backendName() string {
	switch c.backend {
	case BackendXClip:
		return "xclip"
	case BackendXSel:
		return "xsel"
	case BackendWLClipboard:
		return "wl-clipboard"
	default:
		return "disabled"
	}
}

// MonitorChanges starts monitoring for clipboard changes using the optimal method
func (c *LinuxClipboard) MonitorChanges(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
	c.mu.Lock()
	if c.isRunning {
		c.mu.Unlock()
		return
	}
	c.isRunning = true
	c.mu.Unlock()

	if c.backend == BackendDisabled {
		c.logger.Warn("Cannot monitor clipboard - no backend available")
		return
	}

	c.logger.Info("Starting clipboard monitoring", 
		zap.String("backend", c.backendName()),
		zap.String("mode", c.monitorModeName()))

	// Choose monitoring method based on detected mode
	switch c.monitorMode {
	case MonitorModeXFixes:
		go c.monitorWithXFixes(contentCh, stopCh)
	case MonitorModeWayland:
		go c.monitorWithWayland(contentCh, stopCh)
	default:
		go c.monitorWithAdaptivePolling(contentCh, stopCh)
	}
}

// monitorModeName returns human-readable monitoring mode name
func (c *LinuxClipboard) monitorModeName() string {
	switch c.monitorMode {
	case MonitorModeXFixes:
		return "xfixes-events"
	case MonitorModeWayland:
		return "wayland-events"
	default:
		return "adaptive-polling"
	}
}

// monitorWithXFixes uses X11 XFixes extension for event-based monitoring
func (c *LinuxClipboard) monitorWithXFixes(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
	defer func() {
		c.mu.Lock()
		c.isRunning = false
		c.mu.Unlock()
		c.logger.Info("XFixes monitoring stopped")
	}()
	
	c.logger.Info("Starting XFixes event-based monitoring")
	
	// Use xprop to monitor clipboard owner changes
	cmd := exec.Command("xprop", "-root", "-spy", "_NET_SELECTION_OWNER_CHANGES_CLIPBOARD")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		c.logger.Error("Failed to create xprop pipe, falling back to polling", zap.Error(err))
		c.monitorWithAdaptivePolling(contentCh, stopCh)
		return
	}
	
	if err := cmd.Start(); err != nil {
		c.logger.Error("Failed to start xprop, falling back to polling", zap.Error(err))
		c.monitorWithAdaptivePolling(contentCh, stopCh)
		return
	}
	
	c.monitorProc = cmd.Process
	c.logger.Info("XFixes monitoring started", zap.Int("pid", cmd.Process.Pid))
	
	// Monitor output for clipboard change events
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		select {
		case <-stopCh:
			cmd.Process.Kill()
			return
		default:
			// Clipboard change detected
			c.logger.Debug("XFixes: Clipboard change event detected")
			
			content, err := c.Read()
			if err != nil {
				c.logger.Debug("Failed to read clipboard after XFixes event", zap.Error(err))
				continue
			}
			
			// Check if content actually changed using hash
			newHash := c.hashContent(string(content.Data))
			if newHash == c.lastContentHash {
				continue // Same content
			}
			c.lastContentHash = newHash
			
			select {
			case contentCh <- content:
				c.logger.Info("XFixes: New clipboard content sent", zap.Int("size", len(content.Data)))
			case <-stopCh:
				cmd.Process.Kill()
				return
			}
		}
	}
	
	// If scanner stopped, fall back to polling
	if err := scanner.Err(); err != nil {
		c.logger.Warn("XFixes monitoring failed, falling back to polling", zap.Error(err))
	}
	cmd.Process.Kill()
	c.monitorWithAdaptivePolling(contentCh, stopCh)
}

// monitorWithWayland uses wl-paste -w for event-based monitoring
func (c *LinuxClipboard) monitorWithWayland(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
	defer func() {
		c.mu.Lock()
		c.isRunning = false
		c.mu.Unlock()
		c.logger.Info("Wayland monitoring stopped")
	}()
	
	c.logger.Info("Starting Wayland event-based monitoring")
	
	// Use wl-paste -w to watch for clipboard changes
	cmd := exec.Command("wl-paste", "-w", "echo", "CLIPBOARD_CHANGED")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		c.logger.Error("Failed to create wl-paste pipe, falling back to polling", zap.Error(err))
		c.monitorWithAdaptivePolling(contentCh, stopCh)
		return
	}
	
	if err := cmd.Start(); err != nil {
		c.logger.Error("Failed to start wl-paste, falling back to polling", zap.Error(err))
		c.monitorWithAdaptivePolling(contentCh, stopCh)
		return
	}
	
	c.monitorProc = cmd.Process
	c.logger.Info("Wayland monitoring started", zap.Int("pid", cmd.Process.Pid))
	
	// Monitor output for clipboard change events
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		select {
		case <-stopCh:
			cmd.Process.Kill()
			return
		default:
			// Clipboard change detected
			c.logger.Debug("Wayland: Clipboard change event detected")
			
			content, err := c.Read()
			if err != nil {
				c.logger.Debug("Failed to read clipboard after Wayland event", zap.Error(err))
				continue
			}
			
			// Check if content actually changed using hash
			newHash := c.hashContent(string(content.Data))
			if newHash == c.lastContentHash {
				continue // Same content
			}
			c.lastContentHash = newHash
			
			select {
			case contentCh <- content:
				c.logger.Info("Wayland: New clipboard content sent", zap.Int("size", len(content.Data)))
			case <-stopCh:
				cmd.Process.Kill()
				return
			}
		}
	}
	
	// If scanner stopped, fall back to polling
	if err := scanner.Err(); err != nil {
		c.logger.Warn("Wayland monitoring failed, falling back to polling", zap.Error(err))
	}
	cmd.Process.Kill()
	c.monitorWithAdaptivePolling(contentCh, stopCh)
}

// monitorWithAdaptivePolling implements smart polling with format checking optimization
func (c *LinuxClipboard) monitorWithAdaptivePolling(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
	defer func() {
		c.mu.Lock()
		c.isRunning = false
		c.mu.Unlock()
		c.logger.Info("Adaptive polling monitoring stopped")
	}()
	
	c.logger.Info("Starting adaptive polling monitoring", 
		zap.Duration("base_interval", c.baseInterval),
		zap.Duration("max_interval", c.maxInterval))
	
	currentInterval := c.baseInterval
	ticker := time.NewTicker(currentInterval)
	defer ticker.Stop()

	pollCount := 0

	for {
		select {
		case <-stopCh:
			c.logger.Info("Stop signal received, ending monitoring")
			return
		case <-ticker.C:
			pollCount++
			c.logger.Debug("=== POLLING CYCLE START ===", 
				zap.Int("poll_count", pollCount),
				zap.Duration("current_interval", currentInterval))

			// STEP 1: Check formats first (less intrusive than content reading)
			c.logger.Debug("STEP 1: Getting available clipboard formats")
			newFormats, err := c.getAvailableFormats()
			if err != nil {
				c.logger.Debug("Failed to get clipboard formats", zap.Error(err))
				continue
			}
			
			c.logger.Debug("STEP 1 RESULT: Got clipboard formats", 
				zap.Strings("formats", newFormats),
				zap.Duration("interval", currentInterval),
				zap.Int("skip_counter", c.skipCounter),
				zap.Int("inactive_streak", c.inactiveStreak))
			
			// STEP 2: Smart skip logic - but much less aggressive to avoid missing changes
			c.logger.Debug("STEP 2: Checking format changes")
			formatsChanged := !c.formatsEqual(c.lastFormats, newFormats)
			c.logger.Debug("STEP 2 RESULT: Format comparison", 
				zap.Bool("formats_changed", formatsChanged),
				zap.Strings("old_formats", c.lastFormats),
				zap.Strings("new_formats", newFormats))

			if !formatsChanged {
				c.skipCounter++
				c.inactiveStreak++
				
				c.logger.Debug("Formats unchanged, considering skip", 
					zap.Int("skip_counter", c.skipCounter),
					zap.Int("skip_threshold", c.skipThreshold))
				
				// Only skip occasionally and only when we're really sure nothing changed
				if c.skipCounter < c.skipThreshold {
					c.logger.Debug("SKIPPING content read this cycle", 
						zap.String("reason", "formats_unchanged_and_under_threshold"))
					
					// Be much less aggressive about increasing interval - only after many inactive checks
					if c.inactiveStreak > 20 && currentInterval < c.maxInterval {
						oldInterval := currentInterval
						newInterval := time.Duration(float64(currentInterval) * 1.2) // Smaller increase
						if newInterval > c.maxInterval {
							newInterval = c.maxInterval
						}
						if newInterval != currentInterval {
							currentInterval = newInterval
							ticker.Reset(currentInterval)
							c.logger.Debug("Slightly increased polling interval due to long inactivity", 
								zap.Duration("old_interval", oldInterval),
								zap.Duration("new_interval", currentInterval),
								zap.Int("inactive_streak", c.inactiveStreak))
						}
					}
					c.logger.Debug("=== POLLING CYCLE END (SKIPPED) ===")
					continue
				}
				c.skipCounter = 0 // Reset skip counter
				c.logger.Debug("Skip threshold reached, will read content anyway")
			} else {
				// Formats changed, update and reset counters
				c.logger.Info("CLIPBOARD FORMATS CHANGED - This indicates potential new content!", 
					zap.Strings("old_formats", c.lastFormats),
					zap.Strings("new_formats", newFormats))
				c.lastFormats = newFormats
				c.skipCounter = 0
				c.inactiveStreak = 0
			}
			
			// STEP 3: Read content and check for changes
			c.logger.Debug("STEP 3: Reading clipboard content")
			startTime := time.Now()
			content, err := c.Read()
			readDuration := time.Since(startTime)
			
			if err != nil {
				c.logger.Warn("STEP 3 FAILED: Could not read clipboard", 
					zap.Error(err),
					zap.Duration("read_duration", readDuration))
				continue
			}
			
			c.logger.Debug("STEP 3 RESULT: Content read successfully", 
				zap.String("content_type", string(content.Type)),
				zap.Int("content_size", len(content.Data)),
				zap.Duration("read_duration", readDuration),
				zap.String("content_preview", c.getContentPreview(string(content.Data), 100)))
			
			// STEP 4: Hash-based change detection
			c.logger.Debug("STEP 4: Calculating content hash for change detection")
			newHash := c.hashContent(string(content.Data))
			hashChanged := newHash != c.lastContentHash
			
			c.logger.Debug("STEP 4 RESULT: Hash comparison", 
				zap.Bool("hash_changed", hashChanged),
				zap.String("old_hash", c.lastContentHash),
				zap.String("new_hash", newHash))
			
			if !hashChanged {
				c.inactiveStreak++
				c.logger.Debug("Content unchanged (same hash), no action needed", 
					zap.String("hash", newHash),
					zap.Int("inactive_streak", c.inactiveStreak))
				c.logger.Debug("=== POLLING CYCLE END (NO CHANGE) ===")
				continue // No actual change
			}
			
			// STEP 5: Real change detected - reset to base interval and send content
			c.logger.Info("🎉 REAL CLIPBOARD CHANGE DETECTED! 🎉", 
				zap.String("old_hash", c.lastContentHash),
				zap.String("new_hash", newHash),
				zap.String("content_type", string(content.Type)),
				zap.Int("content_size", len(content.Data)),
				zap.Bool("formats_changed", formatsChanged))
			
			c.lastContentHash = newHash
			c.inactiveStreak = 0
			
			if currentInterval != c.baseInterval {
				currentInterval = c.baseInterval
				ticker.Reset(currentInterval)
				c.logger.Debug("Reset polling interval due to activity", zap.Duration("interval", currentInterval))
			}
			
			c.logger.Debug("STEP 5: Sending content to daemon")
			select {
			case contentCh <- content:
				c.logger.Info("✅ CLIPBOARD CONTENT SUCCESSFULLY SENT TO DAEMON", 
					zap.String("type", string(content.Type)),
					zap.Int("size", len(content.Data)))
			case <-stopCh:
				c.logger.Info("Stop signal received while sending content")
				return
			}
			c.logger.Debug("=== POLLING CYCLE END (CONTENT SENT) ===")
		}
	}
}

// Close releases resources
func (c *LinuxClipboard) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isRunning {
		c.cancel()
		c.isRunning = false
	}
	
	// Kill monitoring process if running
	if c.monitorProc != nil {
		c.monitorProc.Kill()
		c.monitorProc = nil
	}
} 