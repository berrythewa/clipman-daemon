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
}

// GetPlatformClipboard returns the appropriate clipboard for the current platform
// The actual implementation is selected at compile time through build tags
func GetPlatformClipboard() Clipboard {
	return NewClipboard()
} 