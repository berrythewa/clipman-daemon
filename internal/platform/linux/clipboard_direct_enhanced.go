//go:build linux
// +build linux

package platform

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"go.uber.org/zap"
)

// EnhancedDirectClipboard combines direct C code access with CLI tool fallbacks
type EnhancedDirectClipboard struct {
	mu             sync.Mutex
	directBackend  *DirectClipboardBackend
	logger         *zap.Logger
	ctx            context.Context
	cancel         context.CancelFunc
	
	// Fallback configuration
	useDirectFirst bool
	fallbackToCLI  bool
	
	// Status tracking
	directAvailable bool
	lastError       string
	errorCount      int
	
	// Monitoring
	isMonitoring    bool
	lastContent     string
	lastContentHash string
}

// NewEnhancedDirectClipboard creates a new enhanced clipboard with direct C access
func NewEnhancedDirectClipboard(logger *zap.Logger) (*EnhancedDirectClipboard, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	ctx, cancel := context.WithCancel(context.Background())
	
	clipboard := &EnhancedDirectClipboard{
		logger:        logger,
		ctx:           ctx,
		cancel:        cancel,
		useDirectFirst: true,
		fallbackToCLI:  true,
	}
	
	// Try to initialize direct backend
	directBackend, err := NewDirectClipboardBackend(logger)
	if err != nil {
		logger.Warn("Failed to initialize direct clipboard backend", zap.Error(err))
		clipboard.directAvailable = false
		clipboard.useDirectFirst = false
	} else {
		logger.Info("✅ Direct clipboard backend initialized successfully")
		clipboard.directBackend = directBackend
		clipboard.directAvailable = true
	}
	
	// Check if CLI tools are available as fallback
	if clipboard.fallbackToCLI {
		clipboard.checkCLITools()
	}
	
	return clipboard, nil
}

// checkCLITools checks availability of CLI clipboard tools
func (c *EnhancedDirectClipboard) checkCLITools() {
	tools := []string{"xclip", "xsel", "wl-copy", "wl-paste"}
	available := make([]string, 0)
	
	for _, tool := range tools {
		if _, err := exec.LookPath(tool); err == nil {
			available = append(available, tool)
		}
	}
	
	if len(available) > 0 {
		c.logger.Info("CLI clipboard tools available for fallback", zap.Strings("tools", available))
	} else {
		c.logger.Warn("No CLI clipboard tools available for fallback")
		c.fallbackToCLI = false
	}
}

// Read reads clipboard content with intelligent backend selection
func (c *EnhancedDirectClipboard) Read() (*types.ClipboardContent, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logger.Debug("Starting clipboard read operation")

	// Try direct backend first if available and preferred
	if c.useDirectFirst && c.directAvailable && c.directBackend != nil {
		content, err := c.directBackend.Read()
		if err == nil {
			c.logger.Debug("Successfully read clipboard via direct backend", 
				zap.String("type", string(content.Type)),
				zap.Int("size", len(content.Data)))
			c.errorCount = 0
			c.lastError = ""
			return content, nil
		}
		
		c.logger.Debug("Direct backend failed, will try fallback", zap.Error(err))
		c.lastError = err.Error()
		c.errorCount++
		
		// If direct backend fails too many times, disable it temporarily
		if c.errorCount > 3 {
			c.logger.Warn("Direct backend failing repeatedly, disabling temporarily")
			c.useDirectFirst = false
		}
	}

	// Fallback to CLI tools if available
	if c.fallbackToCLI {
		content, err := c.readWithCLITools()
		if err == nil {
			c.logger.Debug("Successfully read clipboard via CLI tools", 
				zap.String("type", string(content.Type)),
				zap.Int("size", len(content.Data)))
			
			// Re-enable direct backend if CLI works
			if !c.useDirectFirst && c.directAvailable {
				c.logger.Info("Re-enabling direct backend after successful CLI fallback")
				c.useDirectFirst = true
				c.errorCount = 0
			}
			
			return content, nil
		}
		
		c.logger.Debug("CLI tools also failed", zap.Error(err))
	}

	return nil, fmt.Errorf("all clipboard backends failed: direct=%v, cli=%v", 
		c.directAvailable, c.fallbackToCLI)
}

// readWithCLITools reads clipboard using CLI tools
func (c *EnhancedDirectClipboard) readWithCLITools() (*types.ClipboardContent, error) {
	// Check environment to determine best CLI tool
	if c.isWaylandSession() {
		return c.readWithWaylandCLI()
	} else if c.isX11Session() {
		return c.readWithX11CLI()
	}
	
	// Try both if environment is unclear
	if content, err := c.readWithWaylandCLI(); err == nil {
		return content, nil
	}
	
	return c.readWithX11CLI()
}

// readWithWaylandCLI reads clipboard using Wayland CLI tools
func (c *EnhancedDirectClipboard) readWithWaylandCLI() (*types.ClipboardContent, error) {
	// Try wl-paste
	cmd := exec.Command("wl-paste")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("wl-paste failed: %w", err)
	}
	
	text := strings.TrimSpace(string(output))
	if text == "" {
		return nil, fmt.Errorf("empty clipboard content")
	}
	
	// Detect content type
	contentType := c.detectContentType(text)
	
	return &types.ClipboardContent{
		Type: contentType,
		Data: []byte(text),
	}, nil
}

// readWithX11CLI reads clipboard using X11 CLI tools
func (c *EnhancedDirectClipboard) readWithX11CLI() (*types.ClipboardContent, error) {
	// Try xclip first
	cmd := exec.Command("xclip", "-selection", "clipboard", "-o")
	output, err := cmd.Output()
	if err != nil {
		// Try xsel as fallback
		cmd = exec.Command("xsel", "--clipboard", "--output")
		output, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("both xclip and xsel failed: %w", err)
		}
	}
	
	text := strings.TrimSpace(string(output))
	if text == "" {
		return nil, fmt.Errorf("empty clipboard content")
	}
	
	// Detect content type
	contentType := c.detectContentType(text)
	
	return &types.ClipboardContent{
		Type: contentType,
		Data: []byte(text),
	}, nil
}

// detectContentType detects the type of content from text
func (c *EnhancedDirectClipboard) detectContentType(text string) types.ContentType {
	// Check for URL
	if strings.HasPrefix(text, "http://") || strings.HasPrefix(text, "https://") {
		return types.TypeURL
	}
	
	// Check for file path
	if strings.Contains(text, "/") && !strings.Contains(text, " ") {
		return types.TypeFilePath
	}
	
	// Default to text
	return types.TypeText
}

// Write writes content to clipboard
func (c *EnhancedDirectClipboard) Write(content *types.ClipboardContent) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if content == nil || len(content.Data) == 0 {
		return fmt.Errorf("empty content")
	}

	c.logger.Debug("Starting clipboard write operation", 
		zap.String("type", string(content.Type)),
		zap.Int("size", len(content.Data)))

	// Try direct backend first if available
	if c.directAvailable && c.directBackend != nil {
		err := c.directBackend.Write(content)
		if err == nil {
			c.logger.Debug("Successfully wrote clipboard via direct backend")
			return nil
		}
		
		c.logger.Debug("Direct backend write failed, trying CLI fallback", zap.Error(err))
	}

	// Fallback to CLI tools
	if c.fallbackToCLI {
		return c.writeWithCLITools(content)
	}

	return fmt.Errorf("no available clipboard backend for writing")
}

// writeWithCLITools writes content using CLI tools
func (c *EnhancedDirectClipboard) writeWithCLITools(content *types.ClipboardContent) error {
	text := string(content.Data)
	
	// Check environment to determine best CLI tool
	if c.isWaylandSession() {
		return c.writeWithWaylandCLI(text)
	} else if c.isX11Session() {
		return c.writeWithX11CLI(text)
	}
	
	// Try both if environment is unclear
	if err := c.writeWithWaylandCLI(text); err == nil {
		return nil
	}
	
	return c.writeWithX11CLI(text)
}

// writeWithWaylandCLI writes content using Wayland CLI tools
func (c *EnhancedDirectClipboard) writeWithWaylandCLI(text string) error {
	cmd := exec.Command("wl-copy")
	cmd.Stdin = strings.NewReader(text)
	return cmd.Run()
}

// writeWithX11CLI writes content using X11 CLI tools
func (c *EnhancedDirectClipboard) writeWithX11CLI(text string) error {
	// Try xclip first
	cmd := exec.Command("xclip", "-selection", "clipboard", "-i")
	cmd.Stdin = strings.NewReader(text)
	err := cmd.Run()
	if err != nil {
		// Try xsel as fallback
		cmd = exec.Command("xsel", "--clipboard", "--input")
		cmd.Stdin = strings.NewReader(text)
		err = cmd.Run()
		if err != nil {
			return fmt.Errorf("both xclip and xsel failed: %w", err)
		}
	}
	
	return nil
}

// MonitorChanges monitors clipboard changes
func (c *EnhancedDirectClipboard) MonitorChanges(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
	c.mu.Lock()
	if c.isMonitoring {
		c.mu.Unlock()
		c.logger.Warn("Monitoring already active")
		return
	}
	c.isMonitoring = true
	c.mu.Unlock()

	c.logger.Info("Starting enhanced clipboard monitoring")

	// Use direct backend monitoring if available
	if c.directAvailable && c.directBackend != nil {
		c.directBackend.MonitorChanges(contentCh, stopCh)
		return
	}

	// Fallback to polling
	c.monitorWithPolling(contentCh, stopCh)
}

// monitorWithPolling monitors clipboard changes using polling
func (c *EnhancedDirectClipboard) monitorWithPolling(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
	c.logger.Info("Using polling for clipboard monitoring")
	
	var lastContent string
	
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		
		for {
			select {
			case <-stopCh:
				c.logger.Info("Stopping polling clipboard monitoring")
				c.mu.Lock()
				c.isMonitoring = false
				c.mu.Unlock()
				return
			case <-ticker.C:
				content, err := c.Read()
				if err == nil {
					currentContent := string(content.Data)
					if currentContent != lastContent {
						lastContent = currentContent
						select {
						case contentCh <- content:
							c.logger.Debug("Sent clipboard content via polling")
						case <-stopCh:
							return
						}
					}
				}
			}
		}
	}()
}

// Close cleans up resources
func (c *EnhancedDirectClipboard) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logger.Info("Closing enhanced clipboard")
	
	if c.cancel != nil {
		c.cancel()
	}
	
	if c.directBackend != nil {
		c.directBackend.Close()
		c.directBackend = nil
	}
	
	c.isMonitoring = false
}

// GetMonitoringStatus returns monitoring status
func (c *EnhancedDirectClipboard) GetMonitoringStatus() types.MonitoringStatus {
	c.mu.Lock()
	defer c.mu.Unlock()

	mode := "polling"
	if c.directAvailable && c.directBackend != nil {
		status := c.directBackend.GetMonitoringStatus()
		mode = status.Mode
	}
	
	return types.MonitoringStatus{
		IsRunning:    c.isMonitoring,
		Mode:         mode,
		LastActivity: time.Now(),
		ErrorCount:   c.errorCount,
		LastError:    c.lastError,
	}
}

// RestartMonitoring restarts clipboard monitoring
func (c *EnhancedDirectClipboard) RestartMonitoring(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.logger.Info("Restarting enhanced clipboard monitoring")
	
	// Stop current monitoring
	c.isMonitoring = false
	
	// Restart direct backend if available
	if c.directAvailable && c.directBackend != nil {
		return c.directBackend.RestartMonitoring(contentCh, stopCh)
	}
	
	// Restart polling
	c.isMonitoring = true
	go c.monitorWithPolling(contentCh, stopCh)
	
	return nil
}

// SetStealthMode configures stealth mode (not supported in enhanced version)
func (c *EnhancedDirectClipboard) SetStealthMode(enabled bool) {
	c.logger.Info("Stealth mode configuration", zap.Bool("enabled", enabled))
	// Enhanced version doesn't support stealth mode as it uses direct access
}

// Environment detection helpers
func (c *EnhancedDirectClipboard) isX11Session() bool {
	return c.getEnv("DISPLAY") != "" && c.getEnv("WAYLAND_DISPLAY") == ""
}

func (c *EnhancedDirectClipboard) isWaylandSession() bool {
	return c.getEnv("WAYLAND_DISPLAY") != ""
}

func (c *EnhancedDirectClipboard) getEnv(key string) string {
	return os.Getenv(key)
} 