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

// GetPlatformClipboard returns the appropriate clipboard for the current platform
// The actual implementation is selected at compile time through build tags
func GetPlatformClipboard() Clipboard {
	return NewClipboard()
} 