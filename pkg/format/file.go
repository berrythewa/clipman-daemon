package format

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/berrythewa/clipman-daemon/internal/types"
)

// FormatFile formats file content for display
func FormatFile(content *types.ClipboardContent, opts Options) string {
	var files []string
	if err := json.Unmarshal(content.Data, &files); err == nil {
		return formatFileList(files, opts)
	}
	
	// Fallback to raw string if not JSON
	return string(content.Data)
}

// FormatFilePreview creates a short preview of file content
func FormatFilePreview(content *types.ClipboardContent, maxLen int) string {
	var files []string
	if err := json.Unmarshal(content.Data, &files); err == nil {
		if len(files) == 1 {
			return TruncateText(files[0], maxLen)
		}
		return fmt.Sprintf("[%d files]", len(files))
	}
	
	// Fallback to truncated string
	return TruncateText(string(content.Data), maxLen)
}

// formatFileList formats a list of file paths
func formatFileList(files []string, opts Options) string {
	if len(files) == 0 {
		return "[Empty file list]"
	}
	
	if len(files) == 1 {
		return files[0]
	}
	
	// Show up to 3 files, then summary
	maxShow := 3
	if len(files) <= maxShow {
		return strings.Join(files, "\n")
	}
	
	preview := strings.Join(files[:maxShow], "\n")
	remaining := len(files) - maxShow
	return preview + fmt.Sprintf("\n... and %d more files", remaining)
} 