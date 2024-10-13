package clipboard

import (
	"bytes"
	"encoding/base64"
	"net/url"
	"net/http"
	"strings"

	"github.com/berrythewa/clipman-daemon/internal/types"
)

func detectContentType(data []byte) types.ContentType {
	// Check for empty data
	if len(data) == 0 {
		return types.TypeText
	}

	// Use http.DetectContentType for basic MIME type detection
	mimeType := http.DetectContentType(data)

	// Check if it's a valid URL
	_, err := url.ParseRequestURI(string(data)) // Convert []byte to string for URL parsing
	if err == nil {
		return types.TypeURL
	}

	switch {
	case mimeType == "text/plain":
		return types.TypeText
	case isBase64Image(data):
		return types.TypeImage
	case bytes.Contains(data, []byte("/")) || bytes.Contains(data, []byte("\\")):
		return types.TypeFilePath
	case mimeType == "application/octet-stream":
		// This could be a file, but we can't be sure. Let's assume it's a file.
		return types.TypeFile
	default:
		// If it's not plaintext and not a generic binary, it's likely an image or other media
		return types.TypeImage
	}
}

func isBase64Image(data []byte) bool {
	// Remove data URL prefix if present
	const prefix = "data:image/"
	if bytes.HasPrefix(data, []byte(prefix)) {
		split := bytes.SplitN(data, []byte(","), 2)
		if len(split) < 2 {
			return false
		}
		data = split[1]
	}

	// Decode base64
	decoded, err := base64.StdEncoding.DecodeString(string(data)) // Decode base64 string
	if err != nil {
		return false
	}

	// Check for common image file signatures
	imageSignatures := map[string][]byte{
		"jpeg": {0xFF, 0xD8, 0xFF},
		"png":  {0x89, 0x50, 0x4E, 0x47},
		"gif":  {0x47, 0x49, 0x46, 0x38},
	}

	for _, signature := range imageSignatures {
		if bytes.HasPrefix(decoded, signature) {
			return true
		}
	}

	return false
}
