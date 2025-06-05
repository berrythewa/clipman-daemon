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
		baseInterval:  500 * time.Millisecond,  // 500ms base
		maxInterval:   2000 * time.Millisecond, // 2s max
		skipThreshold: 3, // Skip content checks when formats unchanged
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

	text, err := c.readClipboardText()
	if err != nil {
		return nil, fmt.Errorf("failed to read clipboard: %w", err)
	}

	// Create content from the clipboard text
	content := &types.ClipboardContent{
		Type: types.TypeText,
		Data: []byte(text),
	}

	c.logger.Debug("Read clipboard content",
		zap.String("backend", c.backendName()),
		zap.String("type", string(content.Type)),
		zap.Int("size", len(content.Data)))

	return content, nil
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

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			// STEP 1: Check formats first (less intrusive than content reading)
			newFormats, err := c.getAvailableFormats()
			if err != nil {
				c.logger.Debug("Failed to get clipboard formats", zap.Error(err))
				continue
			}
			
			// STEP 2: Smart skip logic - if formats haven't changed, skip content check most of the time
			if c.formatsEqual(c.lastFormats, newFormats) {
				c.skipCounter++
				c.inactiveStreak++
				
				// Only check content every Nth time when formats unchanged
				if c.skipCounter < c.skipThreshold {
					// Adaptive interval increase for inactivity
					if c.inactiveStreak > 3 && currentInterval < c.maxInterval {
						newInterval := time.Duration(float64(currentInterval) * 1.5)
						if newInterval > c.maxInterval {
							newInterval = c.maxInterval
						}
						if newInterval != currentInterval {
							currentInterval = newInterval
							ticker.Reset(currentInterval)
							c.logger.Debug("Increased polling interval due to inactivity", 
								zap.Duration("interval", currentInterval))
						}
					}
					continue
				}
				c.skipCounter = 0 // Reset skip counter
			} else {
				// Formats changed, update and reset counters
				c.lastFormats = newFormats
				c.skipCounter = 0
				c.inactiveStreak = 0
			}
			
			// STEP 3: Read content and check for changes
			content, err := c.Read()
			if err != nil {
				c.logger.Debug("Failed to read clipboard during polling", zap.Error(err))
				continue
			}
			
			// STEP 4: Hash-based change detection
			newHash := c.hashContent(string(content.Data))
			if newHash == c.lastContentHash {
				c.inactiveStreak++
				continue // No actual change
			}
			
			// STEP 5: Real change detected - reset to base interval and send content
			c.lastContentHash = newHash
			c.inactiveStreak = 0
			
			if currentInterval != c.baseInterval {
				currentInterval = c.baseInterval
				ticker.Reset(currentInterval)
				c.logger.Debug("Reset polling interval due to activity", zap.Duration("interval", currentInterval))
			}
			
			select {
			case contentCh <- content:
				c.logger.Info("Polling: New clipboard content detected and sent", zap.Int("size", len(content.Data)))
			case <-stopCh:
				return
			}
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