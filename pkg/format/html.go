package format

import (
	"github.com/berrythewa/clipman-daemon/internal/types"
)

// FormatHTML formats HTML content for display
func FormatHTML(content *types.ClipboardContent, opts Options) string {
	html := string(content.Data)
	
	// Apply text truncation similar to text content
	if opts.MaxLines > 0 {
		html = TruncateLines(html, opts.MaxLines)
	}
	
	if opts.MaxWidth > 0 {
		html = TruncateText(html, opts.MaxWidth)
	}
	
	return html
}

// FormatHTMLPreview creates a short preview of HTML content
func FormatHTMLPreview(content *types.ClipboardContent, maxLen int) string {
	html := string(content.Data)
	
	// For preview, show truncated HTML
	if len(html) > maxLen {
		html = html[:maxLen] + "..."
	}
	
	return html
} 