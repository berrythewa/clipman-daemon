package format

import (
	"fmt"

	"github.com/berrythewa/clipman-daemon/internal/types"
)

// FormatImage formats image content for display
func FormatImage(content *types.ClipboardContent, opts Options) string {
	size := FormatSize(int64(len(content.Data)))
	return fmt.Sprintf("[Binary image data - %s]", size)
}

// FormatImagePreview creates a short preview of image content
func FormatImagePreview(content *types.ClipboardContent, maxLen int) string {
	size := FormatSize(int64(len(content.Data)))
	return fmt.Sprintf("[Image %s]", size)
} 