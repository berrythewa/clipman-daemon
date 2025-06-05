//go:build linux
// +build linux

package platform

import (
	"github.com/berrythewa/clipman-daemon/internal/types"
)

// Clipboard defines the interface for clipboard operations
// This is a local copy to avoid circular imports
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
// This is a local copy to avoid circular imports
type Daemonizer interface {
	// Daemonize forks the current process and runs it in the background
	// Returns the PID of the new process or an error
	Daemonize(executable string, args []string, workDir string, dataDir string) (int, error)
	
	// IsRunningAsDaemon returns true if the current process is running as a daemon
	IsRunningAsDaemon() bool
} 