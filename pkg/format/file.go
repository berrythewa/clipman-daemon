package format

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/berrythewa/clipman-daemon/internal/types"
)

// FormatFile formats file content for display
func FormatFile(content *types.ClipboardContent, opts Options) string {
	// First, try to decode base64 if needed
	data := content.Data
	if len(data) > 0 {
		dataStr := string(data)
		
		// Check if it looks like base64:
		// 1. Reasonable length (base64 encoded data is usually longer)
		// 2. Only contains base64 characters (A-Z, a-z, 0-9, +, /, =)
		// 3. Proper padding with = at the end
		if len(dataStr) > 20 && len(dataStr)%4 == 0 {
			// Check if all characters are valid base64
			validBase64 := true
			for _, char := range dataStr {
				if !((char >= 'A' && char <= 'Z') || 
					 (char >= 'a' && char <= 'z') || 
					 (char >= '0' && char <= '9') || 
					 char == '+' || char == '/' || char == '=') {
					validBase64 = false
					break
				}
			}
			
			if validBase64 {
				if decoded, err := base64.StdEncoding.DecodeString(dataStr); err == nil {
					// Additional check: decoded data should be reasonable
					if len(decoded) > 0 && len(decoded) < len(dataStr) {
						data = decoded
					}
				}
			}
		}
	}
	
	// Try to parse as JSON file list
	var files []string
	if err := json.Unmarshal(data, &files); err == nil {
		return formatFileList(files, opts)
	}
	
	// Fallback to decoded string if not JSON
	return string(data)
}

// FormatFilePreview creates a short preview of file content
func FormatFilePreview(content *types.ClipboardContent, maxLen int) string {
	// First, try to decode base64 if needed
	data := content.Data
	if len(data) > 0 {
		dataStr := string(data)
		
		// Check if it looks like base64:
		// 1. Reasonable length (base64 encoded data is usually longer)
		// 2. Only contains base64 characters (A-Z, a-z, 0-9, +, /, =)
		// 3. Proper padding with = at the end
		if len(dataStr) > 20 && len(dataStr)%4 == 0 {
			// Check if all characters are valid base64
			validBase64 := true
			for _, char := range dataStr {
				if !((char >= 'A' && char <= 'Z') || 
					 (char >= 'a' && char <= 'z') || 
					 (char >= '0' && char <= '9') || 
					 char == '+' || char == '/' || char == '=') {
					validBase64 = false
					break
				}
			}
			
			if validBase64 {
				if decoded, err := base64.StdEncoding.DecodeString(dataStr); err == nil {
					// Additional check: decoded data should be reasonable
					if len(decoded) > 0 && len(decoded) < len(dataStr) {
						data = decoded
					}
				}
			}
		}
	}
	
	// Try to parse as JSON file list
	var files []string
	if err := json.Unmarshal(data, &files); err == nil {
		return formatFilePreviewList(files, maxLen)
	}
	
	// Fallback: try to extract meaningful info from decoded string
	rawStr := string(data)
	if strings.Contains(rawStr, "/") || strings.Contains(rawStr, "\\") {
		// Looks like a file path, extract just the filename
		return extractFilename(rawStr, maxLen)
	}
	
	return TruncateText(rawStr, maxLen)
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

// formatFilePreviewList creates a preview for a list of files
func formatFilePreviewList(files []string, maxLen int) string {
	if len(files) == 0 {
		return "(empty)"
	}
	
	if len(files) == 1 {
		// Single file: show just filename + directory context if space allows
		return formatSingleFilePreview(files[0], maxLen)
	}
	
	// Multiple files: show smart summary
	return formatMultipleFilesPreview(files, maxLen)
}

// formatSingleFilePreview formats a single file path for preview
func formatSingleFilePreview(filepath string, maxLen int) string {
	filename := extractFilename(filepath, maxLen)
	
	// If we have space, add directory context
	if len(filename) < maxLen-10 {
		dir := extractDirectory(filepath)
		if dir != "" && len(filename)+len(dir)+4 <= maxLen {
			return fmt.Sprintf("%s (in %s)", filename, dir)
		}
	}
	
	return filename
}

// formatMultipleFilesPreview formats multiple files for preview
func formatMultipleFilesPreview(files []string, maxLen int) string {
	// Group by directory or file extension for better context
	commonDir := findCommonDirectory(files)
	
	if commonDir != "" && len(files) <= 5 {
		// Show filenames if they share a common directory
		var filenames []string
		for _, file := range files {
			filenames = append(filenames, extractFilename(file, 20))
		}
		preview := strings.Join(filenames, ", ")
		if len(preview) <= maxLen-len(commonDir)-8 {
			return fmt.Sprintf("%s in %s", preview, commonDir)
		}
	}
	
	// Fallback: show count and first filename
	firstFile := extractFilename(files[0], maxLen-15)
	return fmt.Sprintf("%s + %d more", firstFile, len(files)-1)
}

// extractFilename extracts just the filename from a full path
func extractFilename(filepath string, maxLen int) string {
	// Handle both Unix and Windows paths
	parts := strings.FieldsFunc(filepath, func(c rune) bool {
		return c == '/' || c == '\\'
	})
	
	if len(parts) == 0 {
		return TruncateText(filepath, maxLen)
	}
	
	filename := parts[len(parts)-1]
	return TruncateText(filename, maxLen)
}

// extractDirectory extracts the directory name from a path
func extractDirectory(filepath string) string {
	// Handle both Unix and Windows paths  
	parts := strings.FieldsFunc(filepath, func(c rune) bool {
		return c == '/' || c == '\\'
	})
	
	if len(parts) <= 1 {
		return ""
	}
	
	// Return the parent directory name
	return parts[len(parts)-2]
}

// findCommonDirectory finds common directory among multiple file paths
func findCommonDirectory(files []string) string {
	if len(files) <= 1 {
		return ""
	}
	
	// Extract directories from all files
	var dirs []string
	for _, file := range files {
		dir := extractDirectory(file)
		if dir != "" {
			dirs = append(dirs, dir)
		}
	}
	
	// If most files share the same directory, return it
	if len(dirs) >= len(files)/2 {
		dirCount := make(map[string]int)
		for _, dir := range dirs {
			dirCount[dir]++
		}
		
		// Find most common directory
		maxCount := 0
		commonDir := ""
		for dir, count := range dirCount {
			if count > maxCount {
				maxCount = count
				commonDir = dir
			}
		}
		
		if maxCount >= len(files)/2 {
			return commonDir
		}
	}
	
	return ""
} 