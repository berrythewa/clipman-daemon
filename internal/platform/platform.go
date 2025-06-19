package platform

import (
	"fmt"
	"github.com/berrythewa/clipman-daemon/internal/types"
	"go.uber.org/zap"
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
	
	// GetMonitoringStatus returns the current monitoring status
	GetMonitoringStatus() types.MonitoringStatus
	
	// RestartMonitoring attempts to restart clipboard monitoring
	RestartMonitoring(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) error
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

// ClipboardFactory defines a function that creates a clipboard with a logger
type ClipboardFactory func(*zap.Logger) Clipboard

// Package variables to hold the platform-specific implementations
var (
	defaultClipboard        Clipboard
	defaultDaemonizer       Daemonizer
	clipboardFactory        ClipboardFactory
)

// RegisterClipboard allows platform-specific packages to register their clipboard implementation
func RegisterClipboard(clipboard Clipboard) {
	defaultClipboard = clipboard
}

// RegisterClipboardFactory allows platform-specific packages to register their clipboard factory
func RegisterClipboardFactory(factory ClipboardFactory) {
	clipboardFactory = factory
}

// RegisterDaemonizer allows platform-specific packages to register their daemonizer implementation
func RegisterDaemonizer(daemonizer Daemonizer) {
	defaultDaemonizer = daemonizer
}

// GetPlatformClipboard returns the appropriate clipboard implementation for the current platform
// The actual implementation is selected at compile time through build tags
func GetPlatformClipboard() Clipboard {
	if defaultClipboard == nil {
		panic("no clipboard implementation registered for this platform")
	}
	return defaultClipboard
}

// GetPlatformClipboardWithLogger returns the appropriate clipboard implementation with a logger
func GetPlatformClipboardWithLogger(logger *zap.Logger) Clipboard {
	if logger == nil {
		logger = zap.NewNop()
	}
	
	logger.Info("🔧 Creating platform clipboard", 
		zap.Bool("has_factory", clipboardFactory != nil),
		zap.Bool("has_default", defaultClipboard != nil))
	
	if clipboardFactory != nil {
		logger.Info("🔧 Using clipboard factory")
		clipboard := clipboardFactory(logger)
		logger.Info("✅ Clipboard created via factory", 
			zap.String("type", fmt.Sprintf("%T", clipboard)),
			zap.Bool("is_nil", clipboard == nil))
		return clipboard
	}
	
	// Fallback to default if no factory is registered
	if defaultClipboard == nil {
		logger.Error("❌ No clipboard implementation available")
		panic("no clipboard implementation registered for this platform")
	}
	
	logger.Info("🔧 Using default clipboard implementation", 
		zap.String("type", fmt.Sprintf("%T", defaultClipboard)))
	return defaultClipboard
}

// GetPlatformDaemonizer returns the appropriate daemonizer implementation for the current platform
// The actual implementation is selected at compile time through build tags
func GetPlatformDaemonizer() Daemonizer {
	if defaultDaemonizer == nil {
		panic("no daemonizer implementation registered for this platform")
	}
	return defaultDaemonizer
} 