package format

import (
	"strings"

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

// FormatHTMLText formats extracted HTML text content for display
func FormatHTMLText(content *types.ClipboardContent, opts Options) string {
	text := string(content.Data)
	
	// Apply text truncation
	if opts.MaxLines > 0 {
		text = TruncateLines(text, opts.MaxLines)
	}
	
	if opts.MaxWidth > 0 {
		text = TruncateText(text, opts.MaxWidth)
	}
	
	return text
}

// FormatHTMLPreview creates a short preview of HTML content
func FormatHTMLPreview(content *types.ClipboardContent, maxLen int) string {
	html := string(content.Data)
	
	// For preview, try to extract meaningful text
	extractedText := extractTextFromHTML(html)
	if extractedText != "" && len(extractedText) < len(html) {
		// Use extracted text for preview if it's shorter
		if len(extractedText) > maxLen {
			extractedText = extractedText[:maxLen] + "..."
		}
		return extractedText
	}
	
	// Fallback to truncated HTML
	if len(html) > maxLen {
		html = html[:maxLen] + "..."
	}
	
	return html
}

// extractTextFromHTML is a simplified version for preview purposes
func extractTextFromHTML(html string) string {
	if html == "" {
		return ""
	}
	
	// Remove script and style tags
	html = strings.ReplaceAll(html, "<script", "<!--script")
	html = strings.ReplaceAll(html, "</script>", "script-->")
	html = strings.ReplaceAll(html, "<style", "<!--style")
	html = strings.ReplaceAll(html, "</style>", "style-->")
	
	// Replace common HTML entities
	html = strings.ReplaceAll(html, "&nbsp;", " ")
	html = strings.ReplaceAll(html, "&amp;", "&")
	html = strings.ReplaceAll(html, "&lt;", "<")
	html = strings.ReplaceAll(html, "&gt;", ">")
	
	// Simple tag removal (not comprehensive, just for preview)
	for {
		start := strings.Index(html, "<")
		if start == -1 {
			break
		}
		end := strings.Index(html[start:], ">")
		if end == -1 {
			break
		}
		html = html[:start] + html[start+end+1:]
	}
	
	// Normalize whitespace
	html = strings.TrimSpace(html)
	
	return html
} 