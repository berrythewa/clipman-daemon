package platform

import (
	"github.com/berrythewa/clipman-daemon/internal/types"
)

// Clipboard defines the interface for clipboard operations
// Each platform implements this interface with native clipboard handling
type Clipboard interface {
	// Read returns the current clipboard content
	Read() (*types.ClipboardContent, error)
	
	// Write sets the clipboard content
	Write(*types.ClipboardContent) error
	
	// MonitorChanges starts monitoring for clipboard changes and sends
	// new content to the provided channel when detected
	MonitorChanges(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{})
	
	// Close releases any resources held by the clipboard implementation
	Close()
}

// Daemonizer defines the interface for platform-specific daemonization
// Each platform implements this interface for native daemon process management
type Daemonizer interface {
	// Daemonize forks the current process and runs it in the background
	// Returns the PID of the new process or an error
	Daemonize(executable string, args []string, workDir string, dataDir string) (int, error)
	
	// IsRunningAsDaemon returns true if the current process is running as a daemon
	IsRunningAsDaemon() bool
}

// GetPlatformClipboard returns the appropriate clipboard implementation for the current platform
// The actual implementation is selected at compile time through build tags
func GetPlatformClipboard() Clipboard {
	return NewClipboard()
}

// GetPlatformDaemonizer returns the appropriate daemonizer implementation for the current platform
// The actual implementation is selected at compile time through build tags
func GetPlatformDaemonizer() Daemonizer {
	return NewDaemonizer()
} 