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
	
	// Enhanced base64 detection:
	// 1. Minimum reasonable length (base64 encoded data is usually longer)
	// 2. Only contains valid base64 characters
	// 3. Proper length (must be divisible by 4 for valid base64)
	// 4. Check for reasonable padding
	if len(dataStr) >= 8 && len(dataStr)%4 == 0 {
		// Count padding characters
		paddingCount := 0
		for i := len(dataStr) - 1; i >= 0 && dataStr[i] == '='; i-- {
			paddingCount++
		}
		
		// Base64 can have at most 2 padding characters
		if paddingCount > 2 {
			return data
		}
		
		// Check if all non-padding characters are valid base64
		validBase64 := true
		for i, char := range dataStr {
			// Skip padding at the end
			if char == '=' && i >= len(dataStr)-paddingCount {
				continue
			}
			
			if !((char >= 'A' && char <= 'Z') || 
				 (char >= 'a' && char <= 'z') || 
				 (char >= '0' && char <= '9') || 
				 char == '+' || char == '/' || char == '-' || char == '_') {
				validBase64 = false
				break
			}
		}
		
		if validBase64 {
			// Try both standard and URL-safe base64 decoding
			var decoded []byte
			var err error
			
			// Try standard base64 first
			decoded, err = base64.StdEncoding.DecodeString(dataStr)
			if err != nil {
				// Try URL-safe base64
				decoded, err = base64.URLEncoding.DecodeString(dataStr)
			}
			
			if err == nil && len(decoded) > 0 {
				// Additional heuristics: decoded data should make sense
				// Check if decoded data is printable text or reasonable binary
				if isReasonableDecoded(decoded) {
					return decoded
				}
			}
		}
	}
	
	return data
}

// isReasonableDecoded checks if decoded data looks reasonable
func isReasonableDecoded(data []byte) bool {
	if len(data) == 0 {
		return false
	}
	
	// Check for common patterns that suggest this is meaningful decoded content
	printableCount := 0
	controlCharCount := 0
	
	for _, b := range data {
		if (b >= 32 && b <= 126) || b == '\t' || b == '\n' || b == '\r' {
			// Printable ASCII or common whitespace
			printableCount++
		} else if b < 32 {
			// Control characters
			controlCharCount++
		}
	}
	
	// If most characters are printable, it's likely text
	printableRatio := float64(printableCount) / float64(len(data))
	return printableRatio >= 0.7 // At least 70% printable characters
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
