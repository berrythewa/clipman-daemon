package clipboard

import (
	"github.com/berrythewa/clipman-daemon/internal/platform"
	"github.com/berrythewa/clipman-daemon/internal/types"
)

// Clipboard defines the interface for all clipboard implementations
// across different platforms.
type Clipboard interface {
	// Read returns the current clipboard content
	Read() (*types.ClipboardContent, error)
	
	// Write sets the clipboard content
	Write(*types.ClipboardContent) error
}

// NewClipboard returns a new clipboard implementation for the current platform
// It delegates to the platform package for the actual implementation
func NewClipboard() Clipboard {
	return platform.GetPlatformClipboard()
}
