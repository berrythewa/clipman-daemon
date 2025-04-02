//go:build linux
// +build linux

package platform

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	cliplib "github.com/atotto/clipboard"
	"github.com/berrythewa/clipman-daemon/internal/types"
)

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
}

// ClipboardLogger defines a minimal interface for logging
type ClipboardLogger interface {
	Printf(format string, v ...interface{})
}

// DefaultLogger provides basic logging to stdout
type DefaultLogger struct{}

func (l DefaultLogger) Printf(format string, v ...interface{}) {
	fmt.Printf(format+"\n", v...)
}

// Monitoring modes
const (
	monitorModeXFixes  = "xfixes"  // X11 XFixes extension (event-based)
	monitorModeWayland = "wayland" // Wayland wl-paste (blocking call)
	monitorModePolling = "polling" // Fallback polling strategy
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

// NewClipboard creates a new platform-specific clipboard implementation
func NewClipboard() *LinuxClipboard {
	return &LinuxClipboard{
		logger: DefaultLogger{},
	}
}

// SetLogger allows setting a custom logger
func (c *LinuxClipboard) SetLogger(logger ClipboardLogger) {
	c.logger = logger
}

// Read gets the current clipboard content
func (c *LinuxClipboard) Read() (*types.ClipboardContent, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Try multiple clipboard reading methods in order of preference
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

	return &types.ClipboardContent{
		Type:    types.TypeText,
		Data:    contentBytes,
		Created: time.Now(),
	}, nil
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

	// Check for X11 XFIXES extension
	if isX11Session() && checkXFixesSupport() {
		c.logger.Printf("X11 with XFixes extension detected, using event-based monitoring")
		return monitorModeXFixes
	}
	
	// Check for Wayland
	if isWaylandSession() {
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
		return false
	}

	// Try to run a small script using xprop to check for XFIXES
	cmd := exec.Command("bash", "-c", "xprop -root | grep -q XFIXES")
	return cmd.Run() == nil
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
				_, _ = pipe.Read(buf) // Ignore error - we just care about notification
				close(pipeReadCh)
			}()
			
			// Wait for either a pipe read or stop signal
			select {
			case <-pipeReadCh:
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

// monitorWithWayland uses wl-paste to monitor clipboard changes in Wayland
func (c *LinuxClipboard) monitorWithWayland(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
	go func() {
		// Create a command that uses wl-paste with -w option to monitor changes
		cmd := exec.Command("wl-paste", "-w", "cat")
		
		c.logger.Printf("Starting Wayland clipboard monitoring with wl-paste")
		
		// Create a pipe to capture the output
		stdout, err := cmd.StdoutPipe()
		if err != nil {
			c.logger.Printf("Failed to create wl-paste pipe, falling back to polling: %v", err)
			c.monitorWithAdaptivePolling(contentCh, stopCh)
			return
		}
		
		// Start the command
		if err := cmd.Start(); err != nil {
			c.logger.Printf("Failed to start wl-paste, falling back to polling: %v", err)
			c.monitorWithAdaptivePolling(contentCh, stopCh)
			return
		}
		
		c.logger.Printf("Started wl-paste monitoring with process PID %d", cmd.Process.Pid)
		
		// Make sure to kill the process when we're done
		defer cmd.Process.Kill()
		
		// Create a reader and buffer
		buf := make([]byte, 4096)
		
		// Channel to signal when reading is done
		readCh := make(chan struct{})
		var readData []byte
		
		// Function to start a read operation
		startRead := func() {
			go func() {
				n, err := stdout.Read(buf)
				if err != nil {
					close(readCh)
					return
				}
				readData = buf[:n]
				close(readCh)
			}()
		}
		
		// Start the first read
		startRead()
		
		for {
			// Wait for either read completion or stop signal
			select {
			case <-readCh:
				// If no data was read, it means the pipe was closed
				if len(readData) == 0 {
					c.logger.Printf("Wayland monitoring: wl-paste stream closed, stopping monitor")
					return
				}
				
				// Create clipboard content
				content := &types.ClipboardContent{
					Type:    types.TypeText,
					Data:    readData,
					Created: time.Now(),
				}
				
				// Check if content has changed
				c.mu.Lock()
				changed := !bytes.Equal(content.Data, c.lastContent)
				if changed {
					c.lastContent = make([]byte, len(content.Data))
					copy(c.lastContent, content.Data)
				}
				c.mu.Unlock()
				
				if changed {
					// Send the content to the channel
					select {
					case contentCh <- content:
						c.logger.Printf("Wayland notification: New clipboard content detected and sent (size: %d bytes)", len(content.Data))
					case <-stopCh:
						return
					}
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
		// Configure adaptive polling parameters
		baseInterval := 1 * time.Second
		maxInterval := 10 * time.Second
		currentInterval := baseInterval
		
		// Track inactivity to adjust polling frequency
		c.inactiveStreak = 0
		
		// Create ticker with initial interval
		ticker := time.NewTicker(currentInterval)
		defer ticker.Stop()

		c.logger.Printf("Started adaptive polling with base interval %v, max interval %v", baseInterval, maxInterval)
		
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				// Check clipboard content
				content, err := c.Read()
				if err != nil {
					// If content unchanged, track inactivity
					if err.Error() == "content unchanged" {
						c.inactiveStreak++
						
						// Exponential backoff for inactivity
						if c.inactiveStreak >= 3 {
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

// Write sets the clipboard content
func (c *LinuxClipboard) Write(content *types.ClipboardContent) error {
	if content.Type != types.TypeText {
		return fmt.Errorf("only text content is supported on Linux")
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Try multiple clipboard writing methods in order of preference
	err := c.writeClipboardContent(content.Data)
	
	// Update the lastContent on successful write
	if err == nil {
		c.lastContent = make([]byte, len(content.Data))
		copy(c.lastContent, content.Data)
	}
	
	return err
}

// writeClipboardContent attempts multiple methods to write to the clipboard
func (c *LinuxClipboard) writeClipboardContent(data []byte) error {
	// Try atotto first
	if err := cliplib.WriteAll(string(data)); err == nil {
		return nil
	}
	
	// Try environment-specific writers
	var lastError error
	
	// X11 environment
	if isX11Session() {
		if hasCommand("xclip") {
			cmd := exec.Command("xclip", "-selection", "clipboard")
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
