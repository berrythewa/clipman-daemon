package clipboard

import (
	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/berrythewa/clipman-daemon/internal/platform"
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
}

// NewClipboard returns a platform-specific Clipboard implementation.
func NewClipboard() Clipboard {
    // Implementation delegates to platform package.
	return platform.GetPlatformClipboard()
}