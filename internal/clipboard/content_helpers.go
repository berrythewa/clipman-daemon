package clipboard

import (
	"bytes"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/berrythewa/clipman-daemon/internal/types"
)

// detectContentType examines the content data and determines the most likely content type
// This function is used by clipboard implementations that don't provide native type detection
func detectContentType(data []byte) types.ContentType {
	if len(data) == 0 {
		return types.TypeText
	}

	// First, check if it's a valid URL
	strData := string(data)
	strData = strings.TrimSpace(strData)
	
	if isValidURL(strData) {
		return types.TypeURL
	}

	// Check if it looks like a file path
	if isLikelyFilePath(strData) {
		return types.TypeFilePath
	}

	// Look for image magic bytes at the beginning
	if isImage(data) {
		return types.TypeImage
	}

	// Default to text
	return types.TypeText
}

// isValidURL checks if the given string is a valid URL
func isValidURL(s string) bool {
	u, err := url.Parse(s)
	if err != nil {
		return false
	}
	// A valid URL should have a scheme and host
	return u.Scheme != "" && u.Host != ""
}

// isLikelyFilePath checks if the string looks like a file path
func isLikelyFilePath(s string) bool {
	// Check if it has a file extension
	if ext := filepath.Ext(s); ext != "" {
		return true
	}
	
	// Check if it starts with a path separator or drive letter (for Windows)
	if strings.HasPrefix(s, "/") || strings.HasPrefix(s, "\\") || 
	   (len(s) > 1 && s[1] == ':' && (s[0] >= 'A' && s[0] <= 'Z' || s[0] >= 'a' && s[0] <= 'z')) {
		return true
	}
	
	// Check if it has path separators
	if strings.Contains(s, "/") || strings.Contains(s, "\\") {
		return true
	}
	
	return false
}

// isImage checks if the data has image magic bytes
func isImage(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	
	// Check for common image formats by their magic bytes
	// PNG: 89 50 4E 47
	if bytes.HasPrefix(data, []byte{0x89, 0x50, 0x4E, 0x47}) {
		return true
	}
	
	// JPEG: FF D8 FF
	if bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}) {
		return true
	}
	
	// GIF: GIF87a or GIF89a
	if bytes.HasPrefix(data, []byte("GIF87a")) || bytes.HasPrefix(data, []byte("GIF89a")) {
		return true
	}
	
	// BMP: BM
	if bytes.HasPrefix(data, []byte{0x42, 0x4D}) {
		return true
	}
	
	return false
}

// min returns the minimum of two integers
// Used in various places for safe string slicing
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
} 