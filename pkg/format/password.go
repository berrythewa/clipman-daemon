package format

import (
	"fmt"
	"github.com/berrythewa/clipman-daemon/internal/types"
)

// FormatPassword formats password content securely - never shows actual content
func FormatPassword(content *types.ClipboardContent, opts Options) string {
	if content == nil || len(content.Data) == 0 {
		return ""
	}

	// Never show actual password content
	secureMsg := "[HIDDEN PASSWORD]"
	if opts.UseColors {
		secureMsg = ColorizeIf(secureMsg, Dim, opts.UseColors)
	}
	
	// Add some generic info without revealing content
	length := len(content.Data)
	info := fmt.Sprintf("\n%s\nLength: %d characters", secureMsg, length)
	
	return info
}

// FormatPasswordPreview creates a secure preview that doesn't reveal content
func FormatPasswordPreview(content *types.ClipboardContent, maxLen int) string {
	if content == nil || len(content.Data) == 0 {
		return "[EMPTY]"
	}
	
	// Show only basic info, never the content
	length := len(content.Data)
	return fmt.Sprintf("[PASSWORD - %d chars]", length)
}
