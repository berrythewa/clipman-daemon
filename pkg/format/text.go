package format

import (
	"encoding/base64"
	"strings"

	"github.com/berrythewa/clipman-daemon/internal/types"
)

// decodeIfBase64 tries to decode base64 data, returns original if not base64
func decodeIfBase64(data []byte) []byte {
	if len(data) == 0 {
		return data
	}
	
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
					return decoded
				}
			}
		}
	}
	
	return data
}

// FormatText formats text content for display
func FormatText(content *types.ClipboardContent, opts Options) string {
	if content == nil || len(content.Data) == 0 {
		return ""
	}

	// Try to decode base64 if it looks like base64
	data := decodeIfBase64(content.Data)
	text := string(data)

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
	if content == nil || len(content.Data) == 0 {
		return ""
	}

	// Try to decode base64 if it looks like base64
	data := decodeIfBase64(content.Data)
	text := string(data)
	
	// Replace newlines with spaces for preview
	preview := strings.ReplaceAll(text, "\n", " ")
	preview = strings.ReplaceAll(preview, "\r\n", " ")
	preview = strings.ReplaceAll(preview, "\r", " ")
	
	return TruncateText(preview, maxLen)
}
