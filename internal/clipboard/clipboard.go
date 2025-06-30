package clipboard

import (
	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/berrythewa/clipman-daemon/internal/platform"
	"go.uber.org/zap"
)

// Defines interface for clipboard operations.
type Clipboard interface {
    // Read returns the current clipboard content.
    Read() (*types.ClipboardContent, error)
    // Write sets the clipboard content.
    Write(*types.ClipboardContent) error
    // MonitorChanges watches for clipboard changes and sends new content to contentCh.
    MonitorChanges(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{})
    // Close releases any resources held 
	// by the clipboard implementation.
    Close()
    // GetMonitoringStatus returns the current monitoring status.
    GetMonitoringStatus() types.MonitoringStatus
    // RestartMonitoring attempts to restart clipboard monitoring.
    RestartMonitoring(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) error
}

// NewClipboard returns a platform-specific Clipboard implementation.
func NewClipboard() Clipboard {
    // Implementation delegates to platform package with no-op logger for backwards compatibility.
	return platform.GetPlatformClipboard()
}

// NewClipboardWithLogger returns a platform-specific Clipboard implementation with a logger.
func NewClipboardWithLogger(logger *zap.Logger) Clipboard {
    // Implementation delegates to platform package with proper logger.
	return platform.GetPlatformClipboardWithLogger(logger)
}

// NewClipboardWithConfig returns a platform-specific Clipboard implementation with logger and stealth mode configuration.
func NewClipboardWithConfig(logger *zap.Logger, stealthMode bool) Clipboard {
    // Implementation delegates to platform package with proper logger and stealth mode.
	return platform.GetPlatformClipboardWithConfig(logger, stealthMode)
}