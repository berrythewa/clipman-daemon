//go:build linux
// +build linux

package platform

/*
TODO: Clipboard Format Implementation Summary

- [ ] HTML content support (text/html MIME type)
- [ ] Rich text format support (text/rtf MIME type)
- [ ] Performance optimization for large clipboard contents
- [ ] Image format conversion for broader compatibility
- [ ] In-memory caching to reduce disk I/O
- [ ] Implement proper cleanup for temporary files in edge cases
- [ ] Add more robust error recovery for environment changes
- [ ] Support custom MIME type registration and handling
- [ ] Implement proper unit tests for all clipboard operations
*/

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/url"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
	"syscall"

	cliplib "github.com/atotto/clipboard"
	"github.com/berrythewa/clipman-daemon/internal/types"
	"bufio"
)

// Available monitoring modes
const (
	monitorModePolling = "polling" // Polling mode
	monitorModeXFixes  = "xfixes"  // X11 XFixes extension
	monitorModeWayland = "wayland" // Wayland compositor
	monitorModeMir     = "mir"     // Mir display server
)

// Available clipboard tools
const (
	toolAtotto = "atotto"
	toolXclip  = "xclip"
	toolXsel   = "xsel"
	toolWlPaste = "wl-paste"
	toolMirTool = "mir-tool"
)

// MIME types for clipboard content
const (
	mimeText      = "text/plain"
	mimeUTF8Text  = "text/plain;charset=utf-8"
	mimeImage     = "image/png"
	mimeURI       = "text/uri-list"
	mimeBMP       = "image/bmp"
	mimeJPEG      = "image/jpeg"
	mimeGIF       = "image/gif"
	mimeFilenames = "x-special/gnome-copied-files"
)

// ClipboardLogger defines the interface for clipboard logging
type ClipboardLogger interface {
	Printf(format string, v ...interface{})
}

// DefaultLogger provides a basic implementation of the ClipboardLogger
type DefaultLogger struct{}

// Printf implements the ClipboardLogger interface
func (l DefaultLogger) Printf(format string, v ...interface{}) {
	fmt.Printf(format+"\n", v...)
}

// LinuxClipboard is the Linux-specific clipboard implementation
type LinuxClipboard struct {
	lastContent    []byte
	mu             sync.Mutex
	isRunning      bool
	useXFixes      bool
	xfixesProc     *os.Process
	mirProc        *os.Process
	inactiveStreak int
	monitorMode    string
	logger         ClipboardLogger
	stealthMode    bool               // Reduces clipboard access notifications
	baseInterval   time.Duration      // Base polling interval
	maxInterval    time.Duration      // Maximum polling interval
}

// NewClipboard creates a new platform-specific clipboard implementation
func NewClipboard() *LinuxClipboard {
	return &LinuxClipboard{
		isRunning:      false,
		useXFixes:      false,
		xfixesProc:     nil,
		inactiveStreak: 0,
		monitorMode:    monitorModePolling,
		logger:         DefaultLogger{},
		stealthMode:    false,
		baseInterval:   5 * time.Second,   // 5s default
		maxInterval:    30 * time.Second,  // 30s default
	}
}

// SetLogger allows setting a custom logger
func (c *LinuxClipboard) SetLogger(logger ClipboardLogger) {
	c.logger = logger
}

// SetStealthMode sets the stealth mode option
func (c *LinuxClipboard) SetStealthMode(enabled bool) {
	c.stealthMode = enabled
}

// SetPollingIntervals sets the polling intervals in milliseconds
func (c *LinuxClipboard) SetPollingIntervals(baseMs, maxMs int64) {
	// Don't allow intervals that are too small (bad for performance)
	if baseMs < 1000 {
		baseMs = 1000 // Minimum 1 second
	}
	if maxMs < baseMs {
		maxMs = baseMs * 3 // Ensure max is larger than base
	}
	
	c.baseInterval = time.Duration(baseMs) * time.Millisecond
	c.maxInterval = time.Duration(maxMs) * time.Millisecond
}

// detectContentType tries to determine the content type from the data
func (c *LinuxClipboard) detectContentType(data []byte) types.ContentType {
	// Empty data
	if len(data) == 0 {
		return types.TypeText
	}

	// Try to detect URL
	if isURL(string(data)) {
		c.logger.Printf("Detected URL in clipboard content")
		return types.TypeURL
	}

	// Check if it looks like a file path
	if isFilePath(string(data)) {
		c.logger.Printf("Detected file path in clipboard content")
		return types.TypeFilePath
	}

	// Check if it's a JSON array of file paths
	if isFileList(data) {
		c.logger.Printf("Detected file list in clipboard content")
		return types.TypeFile
	}

	// If we can't determine anything else, assume it's text
	return types.TypeText
}

// isURL checks if a string is a URL
func isURL(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}

	// Use a more permissive URL pattern than Windows
	urlPattern := regexp.MustCompile(`^(https?|ftp)://\S+$`)
	if urlPattern.MatchString(text) {
		// Double check with Go's URL parser
		_, err := url.Parse(text)
		return err == nil
	}
	return false
}

// isFilePath checks if text looks like a file path
func isFilePath(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}

	// Check if it starts with / (Unix path) or has a drive letter (Windows path)
	if strings.HasPrefix(text, "/") {
		// Check if the file exists
		_, err := os.Stat(text)
		return err == nil
	}

	return false
}

// isFileList checks if data is a JSON array of file paths
func isFileList(data []byte) bool {
	var filePaths []string
	if err := json.Unmarshal(data, &filePaths); err != nil {
		return false
	}

	// Must have at least one path
	if len(filePaths) == 0 {
		return false
	}

	// Check if at least the first path exists
	_, err := os.Stat(filePaths[0])
	return err == nil
}

// truncateString truncates a string to the specified length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// Read gets the current clipboard content
func (c *LinuxClipboard) Read() (*types.ClipboardContent, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// First try to detect available formats
	formats, err := c.getAvailableFormats()
	if err != nil {
		c.logger.Printf("Failed to get available formats: %v, falling back to basic text read", err)
		// Fall back to simple text reading if format detection fails
		contentBytes, err := c.readClipboardContent()
		if err != nil {
			return nil, err
		}
		
		// Check if content has changed
		if bytes.Equal(contentBytes, c.lastContent) {
			return nil, fmt.Errorf("content unchanged")
		}
		
		// Update the last content
		c.lastContent = make([]byte, len(contentBytes))
		copy(c.lastContent, contentBytes)

		// Detect content type from the data itself
		contentType := c.detectContentType(contentBytes)
		
		return &types.ClipboardContent{
			Type:    contentType,
			Data:    contentBytes,
			Created: time.Now(),
		}, nil
	}

	// Process formats in priority order
	c.logger.Printf("Available clipboard formats: %v", formats)

	// Try image formats first
	if containsAny(formats, mimeImage, mimeBMP, mimeJPEG, mimeGIF) {
		content, err := c.readImageFormat(formats)
		if err == nil {
			// Check if content has changed
			if bytes.Equal(content.Data, c.lastContent) {
				return nil, fmt.Errorf("content unchanged")
			}
			
			// Update the last content
			c.lastContent = make([]byte, len(content.Data))
			copy(c.lastContent, content.Data)
			
			return content, nil
		}
		c.logger.Printf("Error reading image format: %v", err)
	}

	// Try URI list format (files)
	if contains(formats, mimeURI) || contains(formats, mimeFilenames) {
		content, err := c.readFileFormat(formats)
		if err == nil {
			// Check if content has changed
			if bytes.Equal(content.Data, c.lastContent) {
				return nil, fmt.Errorf("content unchanged")
			}
			
			// Update the last content
			c.lastContent = make([]byte, len(content.Data))
			copy(c.lastContent, content.Data)
			
			return content, nil
		}
		c.logger.Printf("Error reading file format: %v", err)
	}

	// Fall back to text format
	contentBytes, err := c.readClipboardContent()
	if err != nil {
		return nil, err
	}
	
	// Check if content has changed
	if bytes.Equal(contentBytes, c.lastContent) {
		return nil, fmt.Errorf("content unchanged")
	}
	
	// Update the last content
	c.lastContent = make([]byte, len(contentBytes))
	copy(c.lastContent, contentBytes)

	// Detect content type from the data itself
	contentType := c.detectContentType(contentBytes)
	
	return &types.ClipboardContent{
		Type:    contentType,
		Data:    contentBytes,
		Created: time.Now(),
	}, nil
}

// getAvailableFormats returns a list of MIME types available in the clipboard
func (c *LinuxClipboard) getAvailableFormats() ([]string, error) {
	// Try X11 environment first with xclip
	if isX11Session() && hasCommand("xclip") {
		cmd := exec.Command("xclip", "-selection", "clipboard", "-t", "TARGETS", "-o")
		output, err := cmd.Output()
		if err == nil {
			formats := parseXClipFormats(output)
			return formats, nil
		}
	}

	// Try Wayland with wl-paste
	if isWaylandSession() && hasCommand("wl-paste") {
		cmd := exec.Command("wl-paste", "--list-types")
		output, err := cmd.Output()
		if err == nil && len(output) > 0 {
			return parseWaylandFormats(output), nil
		}
	}

	// If we can't determine formats, assume at least text is available
	return []string{mimeText}, fmt.Errorf("could not determine available formats")
}

// parseXClipFormats parses the output of xclip -t TARGETS -o
func parseXClipFormats(output []byte) []string {
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

// parseWaylandFormats parses the output of wl-paste --list-types
func parseWaylandFormats(output []byte) []string {
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

// contains checks if a string is in a slice
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// containsAny checks if any of the items are in the slice
func containsAny(slice []string, items ...string) bool {
	for _, item := range items {
		if contains(slice, item) {
			return true
		}
	}
	return false
}

// readImageFormat reads image data from the clipboard
func (c *LinuxClipboard) readImageFormat(formats []string) (*types.ClipboardContent, error) {
	// Try X11 environment first
	if isX11Session() && hasCommand("xclip") {
		// Determine best format to use
		var format string
		if contains(formats, mimeImage) {
			format = mimeImage
		} else if contains(formats, mimeBMP) {
			format = mimeBMP
		} else if contains(formats, mimeJPEG) {
			format = mimeJPEG
		} else if contains(formats, mimeGIF) {
			format = mimeGIF
		} else {
			return nil, fmt.Errorf("no supported image format available")
		}

		c.logger.Printf("Reading image from clipboard with format: %s", format)
		cmd := exec.Command("xclip", "-selection", "clipboard", "-t", format, "-o")
		output, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("xclip image: %v", err)
		}

		c.logger.Printf("Read image data: %d bytes", len(output))
		return &types.ClipboardContent{
			Type:    types.TypeImage,
			Data:    output,
			Created: time.Now(),
		}, nil
	}

	// Try Wayland
	if isWaylandSession() && hasCommand("wl-paste") {
		c.logger.Printf("Reading image from Wayland clipboard")
		cmd := exec.Command("wl-paste", "--no-newline", "--type", mimeImage)
		output, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("wl-paste image: %v", err)
		}

		c.logger.Printf("Read image data: %d bytes", len(output))
		return &types.ClipboardContent{
			Type:    types.TypeImage,
			Data:    output,
			Created: time.Now(),
		}, nil
	}

	return nil, fmt.Errorf("image clipboard access not available")
}

// readFileFormat reads file URI data from the clipboard
func (c *LinuxClipboard) readFileFormat(formats []string) (*types.ClipboardContent, error) {
	var uriData []byte
	var err error

	// Try X11 environment first
	if isX11Session() && hasCommand("xclip") {
		// Try gnome-copied-files format first, then fall back to uri-list
		format := mimeFilenames
		if !contains(formats, mimeFilenames) && contains(formats, mimeURI) {
			format = mimeURI
		}

		c.logger.Printf("Reading file URI from clipboard with format: %s", format)
		cmd := exec.Command("xclip", "-selection", "clipboard", "-t", format, "-o")
		uriData, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("xclip file URI: %v", err)
		}
	} else if isWaylandSession() && hasCommand("wl-paste") {
		// Try Wayland
		format := mimeFilenames
		if !contains(formats, mimeFilenames) && contains(formats, mimeURI) {
			format = mimeURI
		}

		c.logger.Printf("Reading file URI from Wayland clipboard")
		cmd := exec.Command("wl-paste", "--no-newline", "--type", format)
		uriData, err = cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("wl-paste file URI: %v", err)
		}
	} else {
		return nil, fmt.Errorf("file clipboard access not available")
	}

	// Parse the URI data
	return c.parseURIData(uriData)
}

// parseURIData converts URI data to the appropriate content type
func (c *LinuxClipboard) parseURIData(data []byte) (*types.ClipboardContent, error) {
	dataStr := string(data)
	
	// Check for GNOME format
	gnomePrefix := "copy\n"
	if strings.HasPrefix(dataStr, gnomePrefix) {
		// Extract the actual file paths
		lines := strings.Split(strings.TrimPrefix(dataStr, gnomePrefix), "\n")
		var files []string
		for _, line := range lines {
			if strings.TrimSpace(line) != "" {
				files = append(files, line)
			}
		}

		if len(files) == 1 {
			c.logger.Printf("Single file path: %s", files[0])
			return &types.ClipboardContent{
				Type:    types.TypeFilePath,
				Data:    []byte(files[0]),
				Created: time.Now(),
			}, nil
		} else if len(files) > 1 {
			// Serialize multiple files
			fileJSON, err := json.Marshal(files)
			if err != nil {
				return nil, fmt.Errorf("failed to serialize file list: %v", err)
			}
			c.logger.Printf("Multiple file paths: %d files", len(files))
			return &types.ClipboardContent{
				Type:    types.TypeFile,
				Data:    fileJSON,
				Created: time.Now(),
			}, nil
		}
	}

	// Handle standard URI format
	lines := strings.Split(dataStr, "\n")
	var files []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			// Convert URI to path
			if strings.HasPrefix(line, "file://") {
				path, err := url.QueryUnescape(strings.TrimPrefix(line, "file://"))
				if err == nil {
					files = append(files, path)
				}
			} else {
				files = append(files, line)
			}
		}
	}

	if len(files) == 1 {
		c.logger.Printf("Single file path (URI): %s", files[0])
		return &types.ClipboardContent{
			Type:    types.TypeFilePath,
			Data:    []byte(files[0]),
			Created: time.Now(),
		}, nil
	} else if len(files) > 1 {
		// Serialize multiple files
		fileJSON, err := json.Marshal(files)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize file list: %v", err)
		}
		c.logger.Printf("Multiple file paths (URI): %d files", len(files))
		return &types.ClipboardContent{
			Type:    types.TypeFile,
			Data:    fileJSON,
			Created: time.Now(),
		}, nil
	}

	return nil, fmt.Errorf("no valid file paths found in URI data")
}

// readClipboardContent attempts multiple methods to read the clipboard content
func (c *LinuxClipboard) readClipboardContent() ([]byte, error) {
	// Build a list of functions to try in order
	readers := []func() ([]byte, error){
		c.readWithAtotto,
	}
	
	// Add environment-specific readers based on what's available
	if isX11Session() {
		if hasCommand("xclip") {
			readers = append(readers, c.readWithXClip)
		}
		if hasCommand("xsel") {
			readers = append(readers, c.readWithXSel)
		}
	}
	
	if isWaylandSession() {
		if hasCommand("wl-paste") {
			readers = append(readers, c.readWithWlPaste)
		}
	}
	
	if isMirSession() {
		if hasCommand("mir-tool") {
			readers = append(readers, c.readWithMirTool)
		}
	}
	
	// Try each reader in sequence
	var lastError error
	for _, readFn := range readers {
		content, err := readFn()
		if err == nil {
			return content, nil
		}
		lastError = err
	}
	
	return nil, fmt.Errorf("failed to read clipboard using any available method: %v", lastError)
}

// Individual clipboard reading implementations

func (c *LinuxClipboard) readWithAtotto() ([]byte, error) {
	text, err := cliplib.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("atotto clipboard: %v", err)
	}
	return []byte(text), nil
}

func (c *LinuxClipboard) readWithXClip() ([]byte, error) {
	cmd := exec.Command("xclip", "-selection", "clipboard", "-o")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("xclip: %v", err)
	}
	return output, nil
}

func (c *LinuxClipboard) readWithXSel() ([]byte, error) {
	cmd := exec.Command("xsel", "-b")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("xsel: %v", err)
	}
	return output, nil
}

func (c *LinuxClipboard) readWithWlPaste() ([]byte, error) {
	cmd := exec.Command("wl-paste")
	output, err := cmd.Output()
	if err != nil {
		// wl-paste returns error status 1 if clipboard is empty
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return nil, fmt.Errorf("wl-paste: clipboard is empty")
		}
		return nil, fmt.Errorf("wl-paste: %v", err)
	}
	return output, nil
}

func (c *LinuxClipboard) readWithMirTool() ([]byte, error) {
	// Mir clipboard implementation - this is a simplified example
	// Real implementation would use the appropriate Mir clipboard tool or API
	cmd := exec.Command("mir-tool", "clipboard", "get")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("mir-tool: %v", err)
	}
	return output, nil
}

// detectMonitoringMode determines the best clipboard monitoring method
// based on the current environment
func (c *LinuxClipboard) detectMonitoringMode() string {
	// Log the environment detection
	c.logger.Printf("Detecting best clipboard monitoring method for this environment")

	// Check if both X11 and Wayland are available (common in many distributions)
	if isX11Session() && isWaylandSession() {
		c.logger.Printf("Both X11 and Wayland detected, preferring X11 monitoring")
		
		// Check for X11 XFIXES extension
		if checkXFixesSupport() {
			c.logger.Printf("X11 with XFixes extension detected, using event-based monitoring")
			return monitorModeXFixes
		}
	} else if isX11Session() && checkXFixesSupport() {
		// Only X11 is available with XFixes
		c.logger.Printf("X11 with XFixes extension detected, using event-based monitoring")
		return monitorModeXFixes
	} else if isWaylandSession() {
		// Only Wayland is available
		if hasCommand("wl-paste") {
			c.logger.Printf("Wayland session detected with wl-paste available, using Wayland monitoring")
			return monitorModeWayland
		}
		c.logger.Printf("Wayland session detected but wl-paste not found, falling back to polling")
	}
	
	// Check for Mir display server
	if isMirSession() {
		if hasCommand("mir-tool") {
			c.logger.Printf("Mir display server detected, using Mir monitoring")
			return monitorModeMir
		}
		c.logger.Printf("Mir session detected but mir-tool not found, falling back to polling")
	}
	
	// Log that we're falling back to polling
	c.logger.Printf("No event-based clipboard monitoring available, using adaptive polling")
	return monitorModePolling
}

// Environment detection helpers

func isX11Session() bool {
	return os.Getenv("DISPLAY") != ""
}

func isWaylandSession() bool {
	return os.Getenv("WAYLAND_DISPLAY") != ""
}

func isMirSession() bool {
	return os.Getenv("MIR_SOCKET") != ""
}

func hasCommand(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// checkXFixesSupport checks if XFixes is available
func checkXFixesSupport() bool {
	// Try to run xprop to check if X11 is running
	_, err := exec.Command("xprop", "-root").Output()
	if err != nil {
		fmt.Printf("XFixes check: xprop -root failed: %v\n", err)
		return false
	}
	
	fmt.Printf("XFixes check: xprop -root succeeded, checking for XFIXES\n")
	
	// Try to run a small script using xprop to check for XFIXES
	cmd := exec.Command("bash", "-c", "xprop -root | grep -q XFIXES")
	err = cmd.Run()
	if err != nil {
		fmt.Printf("XFixes check: XFIXES not found: %v\n", err)
	} else {
		fmt.Printf("XFixes check: XFIXES extension found\n")
	}
	
	return err == nil
}

// MonitorChanges monitors for clipboard changes and sends updates to the channel
func (c *LinuxClipboard) MonitorChanges(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
	// Prevent multiple monitoring routines
	if c.isRunning {
		c.logger.Printf("Clipboard monitoring already running")
		return
	}
	c.isRunning = true

	// Detect the best monitoring mode for current environment
	c.monitorMode = c.detectMonitoringMode()
	c.logger.Printf("Selected clipboard monitoring mode: %s", c.monitorMode)

	switch c.monitorMode {
	case monitorModeXFixes:
		go c.monitorWithXFixes(contentCh, stopCh)
	case monitorModeWayland:
		go c.monitorWithWayland(contentCh, stopCh)
	case monitorModeMir:
		go c.monitorWithMir(contentCh, stopCh)
	default:
		go c.monitorWithAdaptivePolling(contentCh, stopCh)
	}
}

// monitorWithXFixes uses X11 XFixes extension to monitor clipboard changes
func (c *LinuxClipboard) monitorWithXFixes(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
	c.logger.Printf("Setting up XFixes monitoring...")
	
	// Try alternative method first if it's on Wayland+X11
	if isWaylandSession() {
		c.logger.Printf("Both X11 and Wayland detected, trying more robust X monitoring")
		
		// Try monitoring with a direct xprop approach first
		if c.tryDirectXpropMonitoring(contentCh, stopCh) {
			c.logger.Printf("Successfully set up direct xprop monitoring")
			return
		}
	}
	
	// Create a named pipe for communication
	pipeName := fmt.Sprintf("/tmp/clipman-xfixes-%d", os.Getpid())
	
	// Clean up any existing pipe
	os.Remove(pipeName)
	
	// Create the pipe
	err := exec.Command("mkfifo", pipeName).Run()
	if err != nil {
		c.logger.Printf("Failed to create pipe, falling back to polling: %v", err)
		go c.monitorWithAdaptivePolling(contentCh, stopCh)
		return
	}
	
	// Make sure to clean up the pipe on exit
	defer os.Remove(pipeName)
	
	// Start the XFixes monitoring script
	// This script uses xprop with the -spy option to monitor clipboard changes
	// and writes to the pipe whenever the clipboard changes
	cmd := exec.Command("bash", "-c", fmt.Sprintf(`
		export DISPLAY=${DISPLAY:-:0}
		xprop -root -spy _NET_SELECTION_OWNER_CHANGES_CLIPBOARD | while read line; do
			echo "change" > %s
		done
	`, pipeName))
	
	// Start the monitoring script
	if err := cmd.Start(); err != nil {
		c.logger.Printf("Failed to start XFixes monitoring, falling back to polling: %v", err)
		go c.monitorWithAdaptivePolling(contentCh, stopCh)
		return
	}
	
	// Store the process for cleanup
	c.xfixesProc = cmd.Process
	c.logger.Printf("Started XFixes monitoring with process PID %d", cmd.Process.Pid)
	
	// Open the pipe for reading
	pipe, err := os.OpenFile(pipeName, os.O_RDONLY, 0)
	if err != nil {
		c.logger.Printf("Failed to open pipe, falling back to polling: %v", err)
		cmd.Process.Kill()
		go c.monitorWithAdaptivePolling(contentCh, stopCh)
		return
	}
	
	// Read buffer
	buf := make([]byte, 64)
	
	// Monitoring loop
	go func() {
		defer pipe.Close()
		defer cmd.Process.Kill()
		
		for {
			// Set up a select to handle both pipe reading and stop signal
			pipeReadCh := make(chan struct{})
			
			// Start a goroutine to read from the pipe
			go func() {
				_, err := pipe.Read(buf) // We care about errors here to detect closed pipes
				if err != nil {
					c.logger.Printf("Error reading from pipe: %v", err)
				}
				close(pipeReadCh)
			}()
			
			// Wait for either a pipe read or stop signal
			select {
			case <-pipeReadCh:
				// Check if the process is still running
				if err := cmd.Process.Signal(syscall.Signal(0)); err != nil {
					c.logger.Printf("XFixes monitoring process has died, falling back to polling")
					c.monitorWithAdaptivePolling(contentCh, stopCh)
					return
				}
				
				// Clipboard change detected, read the content
				content, err := c.Read()
				if err != nil {
					if err.Error() != "content unchanged" {
						c.logger.Printf("Error reading clipboard after XFixes notification: %v", err)
					}
					continue
				}
				
				// Send the content to the channel
				select {
				case contentCh <- content:
					c.logger.Printf("XFixes notification: New clipboard content detected and sent (size: %d bytes)", len(content.Data))
				case <-stopCh:
					return
				}
				
			case <-stopCh:
				return
			}
		}
	}()
}

// tryDirectXpropMonitoring tries a more robust monitoring approach using xprop directly
func (c *LinuxClipboard) tryDirectXpropMonitoring(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) bool {
	c.logger.Printf("Trying direct xprop monitoring...")
	
	// Start xprop in a way that works better in mixed Wayland/X11 environments
	cmd := exec.Command("xprop", "-root", "-spy", "_NET_SELECTION_OWNER_CHANGES_CLIPBOARD")
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		c.logger.Printf("Failed to create stdout pipe: %v", err)
		return false
	}
	
	if err := cmd.Start(); err != nil {
		c.logger.Printf("Failed to start xprop: %v", err)
		return false
	}
	
	c.logger.Printf("Started direct xprop monitoring with PID %d", cmd.Process.Pid)
	
	// Store the process for cleanup
	c.xfixesProc = cmd.Process
	
	// Read from xprop output
	go func() {
		defer cmd.Process.Kill()
		
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			select {
			case <-stopCh:
				return
			default:
				// Each line means a clipboard change
				c.logger.Printf("Direct xprop detected clipboard change")
				
				// Read the new content
				content, err := c.Read()
				if err != nil {
					if err.Error() != "content unchanged" {
						c.logger.Printf("Error reading clipboard after xprop notification: %v", err)
					}
					continue
				}
				
				// Send the content
				select {
				case contentCh <- content:
					c.logger.Printf("Direct xprop: New clipboard content detected and sent (size: %d bytes)", len(content.Data))
				case <-stopCh:
					return
				}
			}
		}
		
		// If we get here, xprop has stopped
		if err := scanner.Err(); err != nil {
			c.logger.Printf("Error reading from xprop: %v", err)
		} else {
			c.logger.Printf("xprop monitoring stopped unexpectedly")
		}
		
		// Fall back to polling
		c.monitorWithAdaptivePolling(contentCh, stopCh)
	}()
	
	return true
}

// monitorWithWayland uses wl-paste to monitor clipboard changes in Wayland
func (c *LinuxClipboard) monitorWithWayland(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
	go func() {
		// First try: Use wl-paste with -n -w echo
		c.logger.Printf("Starting Wayland clipboard monitoring with wl-paste -n -w echo")
		
		// Create a command that uses wl-paste with -w option to monitor changes
		cmd := exec.Command("wl-paste", "-n", "-w", "echo", "CLIPBOARD_CHANGED")
		
		// Create a pipe to capture the output
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			c.logger.Printf("Failed to create wl-paste pipe: %v", err)
			c.logger.Printf("Trying alternative approach...")
			c.tryAlternativeWaylandMonitoring(contentCh, stopCh)
			return
		}
		
		// Start the command
		if err := cmd.Start(); err != nil {
			c.logger.Printf("Failed to start wl-paste: %v", err)
			c.logger.Printf("Trying alternative approach...")
			c.tryAlternativeWaylandMonitoring(contentCh, stopCh)
			return
		}
		
		c.logger.Printf("Started wl-paste monitoring with process PID %d", cmd.Process.Pid)
		
		// Make sure to kill the process when we're done
		defer func() {
			c.logger.Printf("Cleaning up wl-paste process PID %d", cmd.Process.Pid)
			cmd.Process.Kill()
		}()
		
		// Create a reader and buffer
		buf := make([]byte, 4096)
		
		// Channel to signal when reading is done
		readCh := make(chan struct{})
		var readData []byte
		
		// Function to start a read operation
		startRead := func() {
			go func() {
				c.logger.Printf("Waiting for wl-paste to detect clipboard change...")
				n, err := stdout.Read(buf)
				if err != nil {
					c.logger.Printf("Error reading from wl-paste: %v", err)
					close(readCh)
					return
				}
				c.logger.Printf("Read %d bytes from wl-paste", n)
				readData = buf[:n]
				close(readCh)
			}()
		}
		
		// Start the first read
		startRead()
		
		// Check if the stream gets closed immediately (within 2 seconds)
		select {
		case <-readCh:
			if len(readData) == 0 {
				c.logger.Printf("Wayland monitoring: wl-paste stream closed immediately")
				c.logger.Printf("Trying alternative approach...")
				c.tryAlternativeWaylandMonitoring(contentCh, stopCh)
				return
			}
			
			c.logger.Printf("Wayland notification received: '%s'", string(readData))
			
			// Don't process the echo output, just get the content directly
			content, err := c.Read()
			if err == nil {
				select {
				case contentCh <- content:
					c.logger.Printf("Wayland notification: New clipboard content detected and sent (size: %d bytes)", len(content.Data))
				case <-stopCh:
					return
				}
			} else {
				c.logger.Printf("Error reading clipboard after Wayland notification: %v", err)
			}
			
			// Reset for next read
			readCh = make(chan struct{})
			startRead()
			
		case <-time.After(2 * time.Second):
			// If we get here, the stream is open and waiting for events
			c.logger.Printf("Wayland monitoring successfully established and waiting for events")
			
		case <-stopCh:
			return
		}
		
		for {
			// Wait for either read completion or stop signal
			select {
			case <-readCh:
				// If no data was read, it means the pipe was closed
				if len(readData) == 0 {
					c.logger.Printf("Wayland monitoring: wl-paste stream closed")
					c.logger.Printf("Trying alternative approach...")
					c.tryAlternativeWaylandMonitoring(contentCh, stopCh)
					return
				}
				
				c.logger.Printf("Wayland notification received: '%s'", string(readData))
				
				// Don't process the echo output, just get the content directly
				content, err := c.Read()
				if err == nil {
					select {
					case contentCh <- content:
						c.logger.Printf("Wayland notification: New clipboard content detected and sent (size: %d bytes)", len(content.Data))
					case <-stopCh:
						return
					}
				} else {
					c.logger.Printf("Error reading clipboard after Wayland notification: %v", err)
				}
				
				// Reset for next read
				readCh = make(chan struct{})
				startRead()
				
			case <-stopCh:
				return
			}
		}
	}()
}

// tryAlternativeWaylandMonitoring attempts a different approach to Wayland monitoring
func (c *LinuxClipboard) tryAlternativeWaylandMonitoring(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
	// Second try: Use wl-paste -w without -n
	c.logger.Printf("Trying alternative Wayland monitoring with wl-paste -w")
	cmd := exec.Command("wl-paste", "-w", "echo", "CHANGED")
	
	// Create a pipe to capture the output
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		c.logger.Printf("Failed to create pipe for alternative approach: %v", err)
		c.logger.Printf("Falling back to polling")
		c.monitorWithAdaptivePolling(contentCh, stopCh)
		return
	}
	
	// Start the command
	if err := cmd.Start(); err != nil {
		c.logger.Printf("Failed to start alternative approach: %v", err)
		c.logger.Printf("Falling back to polling")
		c.monitorWithAdaptivePolling(contentCh, stopCh)
		return
	}
	
	c.logger.Printf("Started alternative Wayland monitoring with process PID %d", cmd.Process.Pid)
	
	// Make sure to kill the process when we're done
	defer func() {
		c.logger.Printf("Cleaning up wl-paste process PID %d", cmd.Process.Pid)
		cmd.Process.Kill()
	}()
	
	// Create a reader
	scanner := bufio.NewScanner(stdout)
	
	// Create a channel for notifications
	notifyCh := make(chan struct{})
	
	// Start a goroutine to read from the pipe
	go func() {
		for scanner.Scan() {
			// Just notify that we got something
			select {
			case notifyCh <- struct{}{}:
				// Sent notification
			case <-stopCh:
				return
			}
		}
		
		// If we get here, the scanner has stopped
		c.logger.Printf("Alternative Wayland monitoring stopped: %v", scanner.Err())
		c.logger.Printf("Falling back to polling")
		c.monitorWithAdaptivePolling(contentCh, stopCh)
	}()
	
	for {
		select {
		case <-notifyCh:
			// Get clipboard content
			content, err := c.Read()
			if err != nil {
				if err.Error() != "content unchanged" {
					c.logger.Printf("Error reading clipboard: %v", err)
				}
				continue
			}
			
			// Send the content
			select {
			case contentCh <- content:
				c.logger.Printf("Alternative monitoring: New clipboard content detected and sent (size: %d bytes)", len(content.Data))
			case <-stopCh:
				return
			}
			
		case <-stopCh:
			return
		}
	}
}

// monitorWithMir uses Mir display server's tools to monitor clipboard
func (c *LinuxClipboard) monitorWithMir(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
	go func() {
		// Temporary file for communication
		tempFile, err := ioutil.TempFile("", "clipman-mir-monitor-")
		if err != nil {
			c.logger.Printf("Failed to create temp file for Mir monitoring, falling back to polling: %v", err)
			c.monitorWithAdaptivePolling(contentCh, stopCh)
			return
		}
		tempFilePath := tempFile.Name()
		tempFile.Close()
		defer os.Remove(tempFilePath)
		
		c.logger.Printf("Starting Mir clipboard monitoring")
		
		// This command sets up a loop to monitor Mir clipboard changes
		// NOTE: This is a placeholder as there's no actual Mir clipboard monitoring tool
		// In a real implementation, the appropriate Mir-specific command would be used
		cmd := exec.Command("bash", "-c", fmt.Sprintf(`
			while true; do
				mir-tool clipboard watch > %s
				sleep 0.1
			done
		`, tempFilePath))
		
		// Start the monitoring script
		if err := cmd.Start(); err != nil {
			c.logger.Printf("Failed to start Mir monitoring, falling back to polling: %v", err)
			c.monitorWithAdaptivePolling(contentCh, stopCh)
			return
		}
		
		// Store the process for cleanup
		c.mirProc = cmd.Process
		c.logger.Printf("Started Mir monitoring with process PID %d", cmd.Process.Pid)
		
		// Get initial file info to detect changes
		initialStat, err := os.Stat(tempFilePath)
		if err != nil {
			c.logger.Printf("Failed to stat temp file, falling back to polling: %v", err)
			cmd.Process.Kill()
			c.monitorWithAdaptivePolling(contentCh, stopCh)
			return
		}
		
		lastModTime := initialStat.ModTime()
		
		// Setup polling ticker for changes
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		
		for {
			select {
			case <-stopCh:
				cmd.Process.Kill()
				return
				
			case <-ticker.C:
				// Check if the file has changed
				stat, err := os.Stat(tempFilePath)
				if err != nil {
					c.logger.Printf("Error checking temp file: %v", err)
					continue
				}
				
				if stat.ModTime().After(lastModTime) {
					lastModTime = stat.ModTime()
					
					// Read the new content
					content, err := c.Read()
					if err != nil {
						if err.Error() != "content unchanged" {
							c.logger.Printf("Error reading clipboard after Mir notification: %v", err)
						}
						continue
					}
					
					// Send the content to the channel
					select {
					case contentCh <- content:
						c.logger.Printf("Mir notification: New clipboard content detected and sent (size: %d bytes)", len(content.Data))
					case <-stopCh:
						return
					}
				}
			}
		}
	}()
}

// monitorWithAdaptivePolling uses a smart polling strategy that adapts to system activity
func (c *LinuxClipboard) monitorWithAdaptivePolling(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
	go func() {
		// Use configured polling parameters
		baseInterval := c.baseInterval
		maxInterval := c.maxInterval
		currentInterval := baseInterval
		
		// For stealth mode, increase the skip threshold
		skipThreshold := 3
		if c.stealthMode {
			skipThreshold = 5 // Skip more checks in stealth mode
		}
		
		// Track inactivity to adjust polling frequency
		c.inactiveStreak = 0
		
		// Create ticker with initial interval
		ticker := time.NewTicker(currentInterval)
		defer ticker.Stop()

		c.logger.Printf("Started adaptive polling with base interval %v, max interval %v, stealth mode: %t", 
			baseInterval, maxInterval, c.stealthMode)
		
		// Track the last content to avoid triggering clipboard access when nothing changed
		var lastContentHash string
		var formats []string
		var skipCounter int = 0
		
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				// Only check formats first (causes fewer clipboard access notifications)
				newFormats, err := c.getAvailableFormats()
				if err != nil {
					c.logger.Printf("Error checking clipboard formats: %v", err)
					continue
				}
				
				// Check if formats changed, if not increment skip counter
				if formatsEqual(formats, newFormats) {
					// Format didn't change, might still have same content
					c.inactiveStreak++
					skipCounter++
					
					// Only do a full content check every Nth time when formats didn't change
					// This dramatically reduces clipboard access notifications
					if skipCounter < skipThreshold {
						// Exponential backoff for inactivity
						if c.inactiveStreak >= 2 { // Reduced from 3 to 2
							// Increase interval gradually, up to max
							newInterval := currentInterval * 2
							if newInterval > maxInterval {
								newInterval = maxInterval
							}
							
							// Only update if interval changed
							if newInterval != currentInterval {
								c.logger.Printf("Increasing polling interval to %v due to inactivity", newInterval)
								currentInterval = newInterval
								ticker.Reset(currentInterval)
							}
						}
						continue
					}
					
					// Reset skip counter after threshold
					skipCounter = 0
				} else {
					// Formats changed, reset skip counter and update formats
					skipCounter = 0
					formats = newFormats
				}
				
				// Check clipboard content
				content, err := c.Read()
				if err != nil {
					// If content unchanged, track inactivity
					if err.Error() == "content unchanged" {
						c.inactiveStreak++
						
						// Exponential backoff for inactivity
						if c.inactiveStreak >= 2 { // Reduced from 3 to 2
							// Increase interval gradually, up to max
							newInterval := currentInterval * 2
							if newInterval > maxInterval {
								newInterval = maxInterval
							}
							
							// Only update if interval changed
							if newInterval != currentInterval {
								c.logger.Printf("Increasing polling interval to %v due to inactivity", newInterval)
								currentInterval = newInterval
								ticker.Reset(currentInterval)
							}
						}
					} else {
						c.logger.Printf("Error reading clipboard during polling: %v", err)
					}
					continue
				}
				
				// Create simple hash of content to detect changes
				currentHash := hashContent(content.Data)
				
				// If content hash is the same, continue
				if currentHash == lastContentHash {
					c.inactiveStreak++
					continue
				}
				
				// New content detected
				lastContentHash = currentHash
				
				// Content changed, reset to base interval
				if currentInterval != baseInterval {
					c.logger.Printf("Content changed, resetting polling interval to %v", baseInterval)
					currentInterval = baseInterval
					ticker.Reset(currentInterval)
				}
				
				// Reset inactivity counter
				c.inactiveStreak = 0
				
				// Send the content to the channel
				select {
				case contentCh <- content:
					c.logger.Printf("Polling: New clipboard content detected and sent (size: %d bytes)", len(content.Data))
				case <-stopCh:
					return
				}
			}
		}
	}()
}

// hashContent creates a simple hash string of content for change detection
func hashContent(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	
	// Simple checksum-based hash that doesn't need to access clipboard again
	var hash uint32
	for i, b := range data {
		hash = (hash << 5) + hash + uint32(b)
		// Only use first 4KB for hashing large content
		if i > 4096 {
			break
		}
	}
	return fmt.Sprintf("%x", hash)
}

// formatsEqual checks if two string slices contain the same elements (order independent)
func formatsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	
	// Create maps for a and b
	aMap := make(map[string]bool)
	for _, v := range a {
		aMap[v] = true
	}
	
	// Check if all elements in b are in a
	for _, v := range b {
		if !aMap[v] {
			return false
		}
	}
	
	return true
}

// Write sets the clipboard content
func (c *LinuxClipboard) Write(content *types.ClipboardContent) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.logger.Printf("Writing to clipboard: type=%s, size=%d bytes", content.Type, len(content.Data))
	
	var err error
	
	switch content.Type {
	case types.TypeText, types.TypeString, types.TypeURL:
		// All text-based types use the same write mechanism
		err = c.writeTextContent(content.Data)
	case types.TypeImage:
		err = c.writeImageContent(content.Data)
	case types.TypeFilePath:
		err = c.writeFilePathContent(content.Data)
	case types.TypeFile:
		err = c.writeFileListContent(content.Data)
	default:
		return fmt.Errorf("unsupported content type: %s", content.Type)
	}
	
	// Update the lastContent on successful write
	if err == nil {
		c.lastContent = make([]byte, len(content.Data))
		copy(c.lastContent, content.Data)
	}
	
	return err
}

// writeTextContent writes text to the clipboard
func (c *LinuxClipboard) writeTextContent(data []byte) error {
	c.logger.Printf("Writing text to clipboard: %s", truncateString(string(data), 100))
	
	// Try atotto first
	if err := cliplib.WriteAll(string(data)); err == nil {
		return nil
	}
	
	// Try environment-specific writers
	var lastError error
	
	// X11 environment
	if isX11Session() {
		if hasCommand("xclip") {
			cmd := exec.Command("xclip", "-selection", "clipboard", "-t", mimeText)
			cmd.Stdin = strings.NewReader(string(data))
			if err := cmd.Run(); err == nil {
				return nil
			} else {
				lastError = fmt.Errorf("xclip: %v", err)
			}
		}
		
		if hasCommand("xsel") {
			cmd := exec.Command("xsel", "-b", "-i")
			cmd.Stdin = strings.NewReader(string(data))
			if err := cmd.Run(); err == nil {
				return nil
			} else {
				lastError = fmt.Errorf("xsel: %v", err)
			}
		}
	}
	
	// Wayland environment
	if isWaylandSession() && hasCommand("wl-copy") {
		cmd := exec.Command("wl-copy")
		cmd.Stdin = strings.NewReader(string(data))
		if err := cmd.Run(); err == nil {
			return nil
		} else {
			lastError = fmt.Errorf("wl-copy: %v", err)
		}
	}
	
	// Mir environment
	if isMirSession() && hasCommand("mir-tool") {
		cmd := exec.Command("mir-tool", "clipboard", "set")
		cmd.Stdin = strings.NewReader(string(data))
		if err := cmd.Run(); err == nil {
			return nil
		} else {
			lastError = fmt.Errorf("mir-tool: %v", err)
		}
	}
	
	if lastError == nil {
		return fmt.Errorf("no suitable clipboard write method available")
	}
	return lastError
}

// writeImageContent writes image data to the clipboard
func (c *LinuxClipboard) writeImageContent(data []byte) error {
	c.logger.Printf("Writing image to clipboard: %d bytes", len(data))
	
	// Create a temporary file for the image
	tmpFile, err := ioutil.TempFile("", "clipman-image-*.png")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	
	// Write the image data to the temporary file
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write image data: %v", err)
	}
	tmpFile.Close()
	
	// Now copy the image to clipboard based on environment
	if isX11Session() && hasCommand("xclip") {
		cmd := exec.Command("xclip", "-selection", "clipboard", "-t", "image/png", "-i", tmpPath)
		if err := cmd.Run(); err == nil {
			return nil
		} else {
			return fmt.Errorf("xclip image: %v", err)
		}
	}
	
	if isWaylandSession() && hasCommand("wl-copy") {
		cmd := exec.Command("wl-copy", "--type", "image/png", "-f", tmpPath)
		if err := cmd.Run(); err == nil {
			return nil
		} else {
			return fmt.Errorf("wl-copy image: %v", err)
		}
	}
	
	return fmt.Errorf("image clipboard writing not supported in current environment")
}

// writeFilePathContent writes a file path to the clipboard
func (c *LinuxClipboard) writeFilePathContent(data []byte) error {
	filePath := string(data)
	c.logger.Printf("Writing file path to clipboard: %s", filePath)
	
	// Check if the file exists
	if _, err := os.Stat(filePath); err != nil {
		return fmt.Errorf("file does not exist: %s", filePath)
	}
	
	// Format for URI lists
	uri := fmt.Sprintf("file://%s", filePath)
	
	// X11 environment
	if isX11Session() && hasCommand("xclip") {
		// Try GNOME format first
		gnomeFormat := fmt.Sprintf("copy\n%s", filePath)
		cmd := exec.Command("xclip", "-selection", "clipboard", "-t", mimeFilenames)
		cmd.Stdin = strings.NewReader(gnomeFormat)
		if err := cmd.Run(); err == nil {
			// Also set the URI list format for compatibility
			uriCmd := exec.Command("xclip", "-selection", "clipboard", "-t", mimeURI)
			uriCmd.Stdin = strings.NewReader(uri)
			uriCmd.Run() // Ignore error, this is just for compatibility
			return nil
		}
		
		// Fallback to URI list
		cmd = exec.Command("xclip", "-selection", "clipboard", "-t", mimeURI)
		cmd.Stdin = strings.NewReader(uri)
		if err := cmd.Run(); err == nil {
			return nil
		} else {
			return fmt.Errorf("xclip file path: %v", err)
		}
	}
	
	// Wayland environment
	if isWaylandSession() && hasCommand("wl-copy") {
		// Try with GNOME format
		gnomeFormat := fmt.Sprintf("copy\n%s", filePath)
		cmd := exec.Command("wl-copy", "--type", mimeFilenames)
		cmd.Stdin = strings.NewReader(gnomeFormat)
		if err := cmd.Run(); err == nil {
			// Also set the URI format for compatibility
			uriCmd := exec.Command("wl-copy", "--type", mimeURI)
			uriCmd.Stdin = strings.NewReader(uri)
			uriCmd.Run() // Ignore error, this is just for compatibility
			return nil
		}
		
		// Fallback to URI format
		cmd = exec.Command("wl-copy", "--type", mimeURI)
		cmd.Stdin = strings.NewReader(uri)
		if err := cmd.Run(); err == nil {
			return nil
		} else {
			return fmt.Errorf("wl-copy file path: %v", err)
		}
	}
	
	return fmt.Errorf("file path clipboard writing not supported in current environment")
}

// writeFileListContent writes a list of file paths to the clipboard
func (c *LinuxClipboard) writeFileListContent(data []byte) error {
	// Deserialize the file list from JSON
	var filePaths []string
	if err := json.Unmarshal(data, &filePaths); err != nil {
		return fmt.Errorf("invalid file list data: %v", err)
	}
	
	c.logger.Printf("Writing file list to clipboard: %d files", len(filePaths))
	
	// Verify that files exist
	for _, path := range filePaths {
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("file does not exist: %s", path)
		}
	}
	
	// Format for GNOME file list
	var gnomeBuilder strings.Builder
	gnomeBuilder.WriteString("copy\n")
	for _, path := range filePaths {
		gnomeBuilder.WriteString(path + "\n")
	}
	gnomeFormat := gnomeBuilder.String()
	
	// Format for URI list
	var uriBuilder strings.Builder
	for _, path := range filePaths {
		uriBuilder.WriteString(fmt.Sprintf("file://%s\n", path))
	}
	uriFormat := uriBuilder.String()
	
	// X11 environment
	if isX11Session() && hasCommand("xclip") {
		// Try GNOME format first
		cmd := exec.Command("xclip", "-selection", "clipboard", "-t", mimeFilenames)
		cmd.Stdin = strings.NewReader(gnomeFormat)
		if err := cmd.Run(); err == nil {
			// Also set the URI list format for compatibility
			uriCmd := exec.Command("xclip", "-selection", "clipboard", "-t", mimeURI)
			uriCmd.Stdin = strings.NewReader(uriFormat)
			uriCmd.Run() // Ignore error, this is just for compatibility
			return nil
		}
		
		// Fallback to URI list
		cmd = exec.Command("xclip", "-selection", "clipboard", "-t", mimeURI)
		cmd.Stdin = strings.NewReader(uriFormat)
		if err := cmd.Run(); err == nil {
			return nil
		} else {
			return fmt.Errorf("xclip file list: %v", err)
		}
	}
	
	// Wayland environment
	if isWaylandSession() && hasCommand("wl-copy") {
		// Try with GNOME format
		cmd := exec.Command("wl-copy", "--type", mimeFilenames)
		cmd.Stdin = strings.NewReader(gnomeFormat)
		if err := cmd.Run(); err == nil {
			// Also set the URI format for compatibility
			uriCmd := exec.Command("wl-copy", "--type", mimeURI)
			uriCmd.Stdin = strings.NewReader(uriFormat)
			uriCmd.Run() // Ignore error, this is just for compatibility
			return nil
		}
		
		// Fallback to URI format
		cmd = exec.Command("wl-copy", "--type", mimeURI)
		cmd.Stdin = strings.NewReader(uriFormat)
		if err := cmd.Run(); err == nil {
			return nil
		} else {
			return fmt.Errorf("wl-copy file list: %v", err)
		}
	}
	
	return fmt.Errorf("file list clipboard writing not supported in current environment")
}

// writeClipboardContent attempts multiple methods to write to the clipboard
// This is a legacy compatibility function that is now used by writeTextContent
func (c *LinuxClipboard) writeClipboardContent(data []byte) error {
	return c.writeTextContent(data)
}

// Close cleans up any resources
func (c *LinuxClipboard) Close() {
	// Kill the XFixes monitoring process if running
	if c.xfixesProc != nil {
		c.logger.Printf("Stopping XFixes monitoring process (PID %d)", c.xfixesProc.Pid)
		c.xfixesProc.Kill()
		c.xfixesProc = nil
	}
	
	// Kill the Mir monitoring process if running
	if c.mirProc != nil {
		c.logger.Printf("Stopping Mir monitoring process (PID %d)", c.mirProc.Pid)
		c.mirProc.Kill()
		c.mirProc = nil
	}
	
	// Mark as not running
	c.isRunning = false
}
