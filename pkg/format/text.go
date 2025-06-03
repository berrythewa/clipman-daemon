package format

import (
	"github.com/berrythewa/clipman-daemon/internal/types"
	"strings"
)

// FormatText formats text content for display
func FormatText(content *types.ClipboardContent, opts Options) string {
	text := string(content.Data)

	// Handle line limits
	if opts.MaxLines > 0 {
		text = TruncateLines(text, opts.MaxLines)
	}

	// Handle width limits
	if opts.MaxWidth > 0 {
		text = TruncateText(text, opts.MaxWidth)
	}

	return text
}

// FormatTextPreview creates a short preview of text content
func FormatTextPreview(content *types.ClipboardContent, maxLen int) string {
	text := string(content.Data)
	
	// Replace newlines with spaces for preview
	preview := strings.ReplaceAll(text, "\n", " ")
	preview = strings.ReplaceAll(preview, "\r\n", " ")
	preview = strings.ReplaceAll(preview, "\r", " ")
	
	return TruncateText(preview, maxLen)
}
