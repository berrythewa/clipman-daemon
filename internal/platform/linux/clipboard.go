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
	
	// Monitoring status tracking
	lastActivity  time.Time
	errorCount    int
	lastError     string
	currentMode   string
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
		baseInterval:  200 * time.Millisecond,  // 200ms - good balance of responsiveness and CPU usage
		maxInterval:   1000 * time.Millisecond, // 1s max for event fallback
		skipThreshold: 1, // Simplified skip logic
	}
	
	// Detect and setup the best available backend and monitoring mode
	clipboard.detectBackend()
	clipboard.detectMonitoringMode()
	return clipboard
}

// detectBackend determines the best clipboard backend for the current environment
func (c *LinuxClipboard) detectBackend() {
	c.logger.Info("🔍 Detecting clipboard backend", 
		zap.String("display", os.Getenv("DISPLAY")),
		zap.String("wayland_display", os.Getenv("WAYLAND_DISPLAY")))
	
	// Check for pure Wayland environment first
	if os.Getenv("WAYLAND_DISPLAY") != "" && os.Getenv("DISPLAY") == "" {
		c.logger.Debug("🔍 Pure Wayland environment detected, checking wl-clipboard")
		if c.checkCommand("wl-copy") && c.checkCommand("wl-paste") {
			c.backend = BackendWLClipboard
			c.logger.Info("✅ Using Wayland clipboard backend (wl-clipboard) - pure Wayland")
			return
		}
		c.logger.Warn("⚠️ Pure Wayland detected but wl-clipboard not available")
	}
	
	// Check for hybrid X11/Wayland environment (like GNOME on Wayland with XWayland)
	if os.Getenv("WAYLAND_DISPLAY") != "" && os.Getenv("DISPLAY") != "" {
		c.logger.Debug("🔍 Hybrid X11/Wayland environment detected")
		
		// Test both backends to see which works better
		waylandWorks := c.testWaylandBackend()
		x11Works := c.testX11Backend()
		
		if waylandWorks && x11Works {
			// Both work - prefer Wayland for native apps, but note the limitation
			c.backend = BackendWLClipboard
			c.logger.Info("✅ Using Wayland clipboard backend (wl-clipboard) - hybrid environment")
			c.logger.Info("ℹ️ Note: X11 apps will use XWayland clipboard, Wayland apps will use native clipboard")
			return
		} else if x11Works {
			// Only X11 works (common in hybrid setups)
			if c.checkCommand("xclip") {
				c.backend = BackendXClip
				c.logger.Info("✅ Using X11 clipboard backend (xclip) - hybrid environment")
				return
			}
			if c.checkCommand("xsel") {
				c.backend = BackendXSel
				c.logger.Info("✅ Using X11 clipboard backend (xsel) - hybrid environment")
				return
			}
		} else if waylandWorks {
			c.backend = BackendWLClipboard
			c.logger.Info("✅ Using Wayland clipboard backend (wl-clipboard) - hybrid environment")
			return
		}
	}
	
	// Check for pure X11 environment
	if os.Getenv("DISPLAY") != "" && os.Getenv("WAYLAND_DISPLAY") == "" {
		c.logger.Debug("🔍 Pure X11 environment detected, checking X11 clipboard tools")
		if c.checkCommand("xclip") {
			c.backend = BackendXClip
			c.logger.Info("✅ Using X11 clipboard backend (xclip) - pure X11")
			return
		}
		if c.checkCommand("xsel") {
			c.backend = BackendXSel  
			c.logger.Info("✅ Using X11 clipboard backend (xsel) - pure X11")
			return
		}
	}
	
	// Fallback: try any available backend regardless of environment
	c.logger.Debug("🔍 Trying fallback backend detection")
	if c.checkCommand("wl-copy") && c.checkCommand("wl-paste") {
		c.backend = BackendWLClipboard
		c.logger.Info("✅ Using Wayland clipboard backend (wl-clipboard) - fallback")
		return
	}
	if c.checkCommand("xclip") {
		c.backend = BackendXClip
		c.logger.Info("✅ Using X11 clipboard backend (xclip) - fallback")
		return
	}
	if c.checkCommand("xsel") {
		c.backend = BackendXSel  
		c.logger.Info("✅ Using X11 clipboard backend (xsel) - fallback")
		return
	}
	
	// No clipboard tools available
	c.backend = BackendDisabled
	c.logger.Error("❌ No clipboard tools available (xclip, xsel, wl-clipboard). Clipboard access disabled.")
}

// detectMonitoringMode determines the best monitoring strategy
func (c *LinuxClipboard) detectMonitoringMode() {
	c.logger.Info("🔍 Detecting optimal clipboard monitoring method")
	
	// Check for Wayland with actual wl-paste -w support (test it, don't just assume)
	if c.isWaylandSession() && c.checkWaylandWatchSupport() {
		compositor := c.detectWaylandCompositor()
		c.monitorMode = MonitorModeWayland
		c.logger.Info("✅ Using Wayland event-based monitoring (wl-paste -w)",
			zap.String("compositor", compositor))
		return
	}
	
	// If Wayland watch mode is not supported, log why
	if c.isWaylandSession() && c.checkCommand("wl-paste") {
		compositor := c.detectWaylandCompositor()
		c.logger.Info("ℹ️ Wayland detected but watch mode not supported by compositor",
			zap.String("compositor", compositor),
			zap.String("reason", "compositor doesn't support wlroots data-control protocol"))
	}
	
	// Check for X11 with XFixes extension (only if no Wayland)
	if c.isX11Session() && c.checkXFixesSupport() {
		c.monitorMode = MonitorModeXFixes
		c.logger.Info("✅ Using X11 XFixes event-based monitoring (most efficient)")
		return
	}
	
	// Fallback to adaptive polling
	c.monitorMode = MonitorModePolling
	c.logger.Info("✅ Using adaptive polling monitoring (fallback)")
}

// checkCommand checks if a command is available in PATH
func (c *LinuxClipboard) checkCommand(command string) bool {
	_, err := exec.LookPath(command)
	available := err == nil
	c.logger.Debug("🔍 Command check", zap.String("command", command), zap.Bool("available", available))
	return available
}

// isX11Session checks if running in X11 environment
func (c *LinuxClipboard) isX11Session() bool {
	hasDisplay := os.Getenv("DISPLAY") != ""
	c.logger.Debug("🔍 X11 session check", zap.Bool("has_display", hasDisplay))
	return hasDisplay
}

// isWaylandSession checks if running in Wayland environment
func (c *LinuxClipboard) isWaylandSession() bool {
	hasWayland := os.Getenv("WAYLAND_DISPLAY") != ""
	c.logger.Debug("🔍 Wayland session check", zap.Bool("has_wayland", hasWayland))
	return hasWayland
}

// checkWaylandWatchSupport checks if wl-paste -w actually works (not just if command exists)
func (c *LinuxClipboard) checkWaylandWatchSupport() bool {
	if !c.checkCommand("wl-paste") {
		c.logger.Debug("❌ Wayland watch check failed: wl-paste not available")
		return false
	}
	
	// Test if wl-paste -w actually works by trying it briefly
	cmd := exec.Command("timeout", "2s", "wl-paste", "-w", "echo", "TEST")
	output, err := cmd.CombinedOutput()
	outputStr := string(output)
	
	// Check for common error messages that indicate lack of support
	unsupportedMessages := []string{
		"Watch mode requires a compositor that supports the wlroots data-control protocol",
		"protocol not supported",
		"not supported",
		"unsupported",
		"no such protocol",
	}
	
	for _, msg := range unsupportedMessages {
		if strings.Contains(strings.ToLower(outputStr), strings.ToLower(msg)) {
			c.logger.Debug("❌ Wayland watch check failed: compositor doesn't support watch mode", 
				zap.String("error", strings.TrimSpace(outputStr)))
			return false
		}
	}
	
	// If we get here, either it worked or timed out normally
	if err != nil {
		// Check if it's just a timeout (which is expected)
		if strings.Contains(err.Error(), "exit status 124") {
			c.logger.Debug("✅ Wayland watch check passed: command timed out normally (expected)")
			return true
		}
		c.logger.Debug("❌ Wayland watch check failed: unexpected error", zap.Error(err))
		return false
	}
	
	c.logger.Debug("✅ Wayland watch check passed: command executed successfully")
	return true
}

// checkXFixesSupport checks if X11 XFixes extension is available
func (c *LinuxClipboard) checkXFixesSupport() bool {
	if !c.checkCommand("xprop") {
		c.logger.Debug("❌ XFixes check failed: xprop not available")
		return false
	}
	
	// Try to run xprop to verify X11 connectivity
	cmd := exec.Command("xprop", "-root", "-notype", "_NET_SUPPORTED")
	err := cmd.Run()
	if err != nil {
		c.logger.Debug("❌ XFixes check failed: xprop command failed", zap.Error(err))
		return false
	}
	
	// Check if the specific XFixes atom exists (this is the key fix)
	cmd = exec.Command("xprop", "-root", "-notype", "_NET_SELECTION_OWNER_CHANGES_CLIPBOARD")
	output, err := cmd.Output()
	if err != nil {
		c.logger.Debug("❌ XFixes check failed: required atom not available", zap.Error(err))
		return false
	}
	
	// Check if the atom exists (should not return "no such atom")
	if strings.Contains(string(output), "no such atom") {
		c.logger.Debug("❌ XFixes check failed: _NET_SELECTION_OWNER_CHANGES_CLIPBOARD atom not found")
		return false
	}
	
	c.logger.Debug("✅ XFixes check passed: xprop command succeeded and required atom exists")
	return true
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
		c.logger.Warn("🔴 MonitorChanges called but already running - ignoring duplicate call")
		return
	}
	c.isRunning = true
	
	// Initialize status tracking
	c.lastActivity = time.Now()
	c.errorCount = 0
	c.lastError = ""
	c.lastContentHash = ""
	
	c.mu.Unlock()

	if c.backend == BackendDisabled {
		c.logger.Warn("Cannot monitor clipboard - no backend available")
		c.mu.Lock()
		c.isRunning = false
		c.updateMonitoringStatus("disabled", fmt.Errorf("no backend available"))
		c.mu.Unlock()
		return
	}

	c.logger.Info("🚀 Starting clipboard monitoring", 
		zap.String("backend", c.backendName()),
		zap.String("mode", c.monitorModeName()),
		zap.String("display", os.Getenv("DISPLAY")),
		zap.String("wayland_display", os.Getenv("WAYLAND_DISPLAY")))

	c.logger.Info("Initialized clipboard monitoring (hash will be set on first content)")

	// Choose monitoring method based on detected mode - NO FALLBACKS IN EVENT METHODS
	switch c.monitorMode {
	case MonitorModeXFixes:
		c.logger.Info("Attempting XFixes monitoring...")
		if c.tryEventBasedMonitoring(contentCh, stopCh, "xfixes") {
			c.mu.Lock()
			c.currentMode = "xfixes-events"
			c.mu.Unlock()
			return // Event monitoring succeeded
		}
		c.logger.Warn("XFixes monitoring failed, using polling")
		fallthrough
	case MonitorModeWayland:
		if c.monitorMode == MonitorModeWayland {
			c.logger.Info("Attempting Wayland monitoring...")
			if c.tryEventBasedMonitoring(contentCh, stopCh, "wayland") {
				c.mu.Lock()
				c.currentMode = "wayland-events"
				c.mu.Unlock()
				return // Event monitoring succeeded
			}
			c.logger.Warn("Wayland monitoring failed, using polling")
		}
		fallthrough
	default:
		c.logger.Info("Using polling monitoring")
		c.mu.Lock()
		c.currentMode = "polling"
		c.mu.Unlock()
		c.monitorWithSimplePolling(contentCh, stopCh)
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
		return "simple-polling"
	}
}

// monitorWithSimplePolling implements reliable polling without complex optimizations
func (c *LinuxClipboard) monitorWithSimplePolling(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
	defer func() {
		c.mu.Lock()
		c.isRunning = false
		c.mu.Unlock()
		c.logger.Info("Simple polling monitoring stopped")
	}()
	
	c.logger.Info("Starting simple polling monitoring", 
		zap.Duration("interval", c.baseInterval))
	
	// Use fixed interval for reliability
	ticker := time.NewTicker(c.baseInterval)
	defer ticker.Stop()

	// Check clipboard availability first
	if !c.checkClipboardAvailable() {
		c.logger.Error("Clipboard not available for monitoring")
		return
	}

	pollCount := 0
	consecutiveErrors := 0
	maxConsecutiveErrors := 5
	
	c.logger.Info("Simple polling started - will poll every", 
		zap.Duration("interval", c.baseInterval),
		zap.String("current_hash", c.lastContentHash))
	
	for {
		select {
		case <-stopCh:
			c.logger.Info("🛑 Stop signal received, ending monitoring")
			return
		case <-ticker.C:
			pollCount++
			if pollCount <= 5 || pollCount%50 == 0 { // Log first 5 polls and every 50th
				c.logger.Info("Polling clipboard", 
					zap.Int("poll_count", pollCount),
					zap.Int("consecutive_errors", consecutiveErrors),
					zap.String("current_hash", c.lastContentHash))
			} else {
				c.logger.Debug("Polling clipboard", zap.Int("poll_count", pollCount))
			}

			// Read content directly - simple and reliable
			content, err := c.Read()
			if err != nil {
				consecutiveErrors++
				c.logger.Warn("Failed to read clipboard", 
					zap.Error(err), 
					zap.Int("consecutive_errors", consecutiveErrors),
					zap.Int("poll_count", pollCount))
				
				if consecutiveErrors >= maxConsecutiveErrors {
					c.logger.Error("Too many consecutive clipboard read errors, checking availability")
					if !c.checkClipboardAvailable() {
						c.logger.Error("Clipboard no longer available, stopping monitoring")
						return
					}
					consecutiveErrors = 0 // Reset after availability check
				}
				continue
			}
			consecutiveErrors = 0 // Reset on successful read
			
			// Hash-based change detection (simple and fast)
			newHash := c.hashContent(string(content.Data))
			
			if pollCount <= 5 { // Log details for first few polls
				c.logger.Info("Content read successfully", 
					zap.Int("poll_count", pollCount),
					zap.String("type", string(content.Type)),
					zap.Int("size", len(content.Data)),
					zap.String("old_hash", c.lastContentHash),
					zap.String("new_hash", newHash),
					zap.Bool("hash_changed", newHash != c.lastContentHash))
			}
			
			if newHash == c.lastContentHash {
				if pollCount <= 5 {
					c.logger.Info("Content unchanged (same hash)", zap.Int("poll_count", pollCount))
				}
				continue // No change
			}
			
			c.logger.Info("Clipboard change detected via polling", 
				zap.String("type", string(content.Type)),
				zap.Int("size", len(content.Data)),
				zap.String("old_hash", c.lastContentHash),
				zap.String("new_hash", newHash),
				zap.Int("poll_count", pollCount))
			
			c.lastContentHash = newHash
			
			// Send content to daemon
			select {
			case contentCh <- content:
				c.logger.Info("Polling: Content sent to daemon successfully", 
					zap.String("type", string(content.Type)),
					zap.Int("size", len(content.Data)),
					zap.String("hash", newHash))
			case <-stopCh:
				c.logger.Info("🛑 Stop signal received while sending content")
				return
			}
		}
	}
}

// checkClipboardAvailable verifies clipboard can be accessed
func (c *LinuxClipboard) checkClipboardAvailable() bool {
	var cmd *exec.Cmd
	
	switch c.backend {
	case BackendXClip:
		// Test xclip access
		cmd = exec.Command("xclip", "-selection", "clipboard", "-o")
	case BackendXSel:
		// Test xsel access  
		cmd = exec.Command("xsel", "--clipboard", "--output")
	case BackendWLClipboard:
		// Test wl-paste access
		cmd = exec.Command("wl-paste", "--no-newline")
	default:
		c.logger.Warn("No clipboard backend available")
		return false
	}
	
	// Try to read clipboard (don't care about content, just access)
	output, err := cmd.Output()
	
	// Consider it available if:
	// 1. No error (success)
	// 2. Exit status 1 with empty output (empty clipboard)
	// 3. Exit status 1 but we got some output (some clipboard tools return 1 but still work)
	available := err == nil || 
		(err != nil && (strings.Contains(err.Error(), "exit status 1") || len(output) > 0))
	
	if available {
		c.logger.Info("Clipboard availability check passed", 
			zap.String("backend", c.backendName()),
			zap.Bool("had_error", err != nil),
			zap.Int("output_length", len(output)))
	} else {
		c.logger.Error("Clipboard availability check failed", 
			zap.String("backend", c.backendName()),
			zap.Error(err),
			zap.Int("output_length", len(output)))
	}
	
	return available
}

// tryEventBasedMonitoring attempts to start event-based monitoring and returns success status
func (c *LinuxClipboard) tryEventBasedMonitoring(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}, mode string) bool {
	// Check clipboard availability first
	if !c.checkClipboardAvailable() {
		c.logger.Error("Clipboard not available for event monitoring")
		return false
	}
	
	var cmd *exec.Cmd
	var cmdName string
	
	switch mode {
	case "xfixes":
		cmd = exec.Command("xprop", "-root", "-spy", "_NET_SELECTION_OWNER_CHANGES_CLIPBOARD")
		cmdName = "xprop"
	case "wayland":
		cmd = exec.Command("wl-paste", "-w", "echo", "CLIPBOARD_CHANGED")
		cmdName = "wl-paste"
	default:
		return false
	}
	
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		c.logger.Error("Failed to create pipe for event monitoring", 
			zap.String("mode", mode), zap.Error(err))
		return false
	}
	
	if err := cmd.Start(); err != nil {
		c.logger.Error("Failed to start event monitoring command", 
			zap.String("mode", mode), zap.String("cmd", cmdName), zap.Error(err))
		return false
	}
	
	c.monitorProc = cmd.Process
	c.logger.Info("Event-based monitoring started successfully", 
		zap.String("mode", mode), zap.Int("pid", cmd.Process.Pid))
	
	// Start monitoring in goroutine
	go func() {
		defer func() {
			c.mu.Lock()
			c.isRunning = false
			if c.monitorProc != nil {
				c.monitorProc.Kill()
				c.monitorProc = nil
			}
			c.updateMonitoringStatus(mode+"-stopped", fmt.Errorf("event monitoring stopped"))
			c.mu.Unlock()
			c.logger.Info("Event monitoring stopped", zap.String("mode", mode))
		}()
		
		c.logger.Info("Event monitoring goroutine started", zap.String("mode", mode))
		
		scanner := bufio.NewScanner(stdout)
		scannerErrors := 0
		maxScannerErrors := 3
		
		for scanner.Scan() {
			select {
			case <-stopCh:
				c.logger.Info("Stop signal received in event monitoring", zap.String("mode", mode))
				cmd.Process.Kill()
				return
			default:
				c.logger.Debug("Event detected", zap.String("mode", mode))
				
				// Update activity timestamp
				c.mu.Lock()
				c.lastActivity = time.Now()
				c.mu.Unlock()
				
				content, err := c.Read()
				if err != nil {
					c.logger.Debug("Failed to read clipboard after event", 
						zap.String("mode", mode), zap.Error(err))
					
					c.mu.Lock()
					c.updateMonitoringStatus(mode, err)
					c.mu.Unlock()
					continue
				}
				
				// Check if content actually changed using hash
				newHash := c.hashContent(string(content.Data))
				if newHash == c.lastContentHash {
					c.logger.Debug("Event fired but content unchanged", zap.String("mode", mode))
					continue // Same content
				}
				c.lastContentHash = newHash
				
				c.logger.Info("Clipboard change detected via event", 
					zap.String("mode", mode),
					zap.String("type", string(content.Type)),
					zap.Int("size", len(content.Data)),
					zap.String("hash", newHash))
				
				select {
				case contentCh <- content:
					c.logger.Info("Event-based: Content sent to daemon", 
						zap.String("mode", mode),
						zap.String("type", string(content.Type)),
						zap.Int("size", len(content.Data)))
					
					// Reset scanner error count on successful operation
					scannerErrors = 0
				case <-stopCh:
					c.logger.Info("Stop signal received while sending content", zap.String("mode", mode))
					cmd.Process.Kill()
					return
				}
			}
		}
		
		// Scanner stopped - check why and potentially restart
		if err := scanner.Err(); err != nil {
			scannerErrors++
			c.logger.Warn("Event monitoring scanner error", 
				zap.String("mode", mode), 
				zap.Error(err),
				zap.Int("error_count", scannerErrors))
			
			c.mu.Lock()
			c.updateMonitoringStatus(mode, err)
			c.mu.Unlock()
			
			// If we haven't had too many errors, try to restart
			if scannerErrors < maxScannerErrors {
				c.logger.Info("Attempting to restart event monitoring after scanner error",
					zap.String("mode", mode),
					zap.Int("attempt", scannerErrors))
				
				cmd.Process.Kill()
				time.Sleep(1 * time.Second) // Brief delay before restart
				
				// Try to restart the event monitoring
				if c.tryEventBasedMonitoring(contentCh, stopCh, mode) {
					c.logger.Info("Successfully restarted event monitoring", zap.String("mode", mode))
					return // New goroutine will handle monitoring
				} else {
					c.logger.Warn("Failed to restart event monitoring", zap.String("mode", mode))
				}
			}
		} else {
			c.logger.Info("Event monitoring ended normally", zap.String("mode", mode))
		}
		
		cmd.Process.Kill()
		
		// If we get here, event monitoring failed - signal that we need supervision
		c.mu.Lock()
		c.updateMonitoringStatus(mode+"-failed", fmt.Errorf("event monitoring failed after %d errors", scannerErrors))
		c.mu.Unlock()
		
		c.logger.Error("Event monitoring failed permanently", 
			zap.String("mode", mode),
			zap.Int("scanner_errors", scannerErrors))
	}()
	
	return true // Successfully started
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

// GetMonitoringStatus returns the current monitoring status
func (c *LinuxClipboard) GetMonitoringStatus() types.MonitoringStatus {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	return types.MonitoringStatus{
		IsRunning:    c.isRunning,
		Mode:         c.currentMode,
		LastActivity: c.lastActivity,
		ErrorCount:   c.errorCount,
		LastError:    c.lastError,
	}
}

// RestartMonitoring attempts to restart clipboard monitoring
func (c *LinuxClipboard) RestartMonitoring(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.logger.Info("Attempting to restart clipboard monitoring",
		zap.String("current_mode", c.currentMode),
		zap.Int("error_count", c.errorCount))
	
	// Stop current monitoring if running
	if c.isRunning {
		c.isRunning = false
		if c.monitorProc != nil {
			c.monitorProc.Kill()
			c.monitorProc = nil
		}
	}
	
	// Reset error tracking
	c.lastError = ""
	
	// Try to restart
	c.isRunning = true
	c.lastActivity = time.Now()
	
	// Release lock before starting monitoring
	c.mu.Unlock()
	
	// Try event monitoring first, then fall back to polling
	if c.monitorMode == MonitorModeXFixes {
		if c.tryEventBasedMonitoring(contentCh, stopCh, "xfixes") {
			c.mu.Lock()
			c.currentMode = "xfixes-events"
			c.mu.Unlock()
			c.logger.Info("Successfully restarted XFixes monitoring")
			return nil
		}
	} else if c.monitorMode == MonitorModeWayland {
		if c.tryEventBasedMonitoring(contentCh, stopCh, "wayland") {
			c.mu.Lock()
			c.currentMode = "wayland-events"
			c.mu.Unlock()
			c.logger.Info("Successfully restarted Wayland monitoring")
			return nil
		}
	}
	
	// Fall back to polling
	c.logger.Info("Event monitoring failed, falling back to polling")
	c.mu.Lock()
	c.currentMode = "polling"
	c.mu.Unlock()
	go c.monitorWithSimplePolling(contentCh, stopCh)
	
	return nil
}

// updateMonitoringStatus updates the monitoring status (call with lock held)
func (c *LinuxClipboard) updateMonitoringStatus(mode string, err error) {
	c.lastActivity = time.Now()
	c.currentMode = mode
	
	if err != nil {
		c.errorCount++
		c.lastError = err.Error()
		c.logger.Warn("Monitoring error occurred",
			zap.String("mode", mode),
			zap.Error(err),
			zap.Int("error_count", c.errorCount))
	}
}

// testWaylandBackend tests if Wayland clipboard backend actually works
func (c *LinuxClipboard) testWaylandBackend() bool {
	if !c.checkCommand("wl-paste") || !c.checkCommand("wl-copy") {
		return false
	}
	
	// Try to read from clipboard (don't care about content)
	cmd := exec.Command("wl-paste", "--no-newline")
	err := cmd.Run()
	
	// Consider it working if no error or just empty clipboard error
	return err == nil || strings.Contains(err.Error(), "exit status 1")
}

// testX11Backend tests if X11 clipboard backend actually works
func (c *LinuxClipboard) testX11Backend() bool {
	if c.checkCommand("xclip") {
		cmd := exec.Command("xclip", "-selection", "clipboard", "-o")
		err := cmd.Run()
		return err == nil || strings.Contains(err.Error(), "exit status 1")
	}
	
	if c.checkCommand("xsel") {
		cmd := exec.Command("xsel", "--clipboard", "--output")
		err := cmd.Run()
		return err == nil || strings.Contains(err.Error(), "exit status 1")
	}
	
	return false
}

// detectWaylandCompositor attempts to identify the Wayland compositor
func (c *LinuxClipboard) detectWaylandCompositor() string {
	// Check common environment variables and processes
	compositors := map[string][]string{
		"GNOME": {"gnome-shell", "mutter"},
		"Sway": {"sway"},
		"Hyprland": {"Hyprland"},
		"River": {"river"},
		"Weston": {"weston"},
		"KDE Plasma": {"plasmashell", "kwin_wayland"},
		"Enlightenment": {"enlightenment"},
	}
	
	for name, processes := range compositors {
		for _, proc := range processes {
			cmd := exec.Command("pgrep", proc)
			if cmd.Run() == nil {
				return name
			}
		}
	}
	
	// Check for common environment variables
	if os.Getenv("GNOME_DESKTOP_SESSION_ID") != "" {
		return "GNOME"
	}
	if os.Getenv("KDEWM") != "" {
		return "KDE"
	}
	if os.Getenv("SWAYSOCK") != "" {
		return "Sway"
	}
	if os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") != "" {
		return "Hyprland"
	}
	
	return "Unknown"
} 