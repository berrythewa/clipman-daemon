package clipboard

import (
	"bytes"
	"encoding/base64"

	// "net/http"
	"log"
	"net/url"
	"strings"

	// "strings"

	"github.com/berrythewa/clipman-daemon/internal/types"
)

func detectContentType(data []byte) types.ContentType {
	if len(data) == 0 {
		return types.TypeText
	}

	// Debug log
	log.Printf("Detecting content type for: %q", string(data))

	// Try to detect image data
	if bytes.HasPrefix(data, []byte{0x89, 'P', 'N', 'G'}) ||
		bytes.HasPrefix(data, []byte{0xFF, 0xD8, 0xFF}) || // JPEG
		bytes.HasPrefix(data, []byte("GIF")) {
		return types.TypeImage
	}

	// Check if it's a valid URL
	if _, err := url.ParseRequestURI(string(data)); err == nil {
		if strings.HasPrefix(string(data), "http") {
			return types.TypeURL
		}
	}

	// Check for file path - but ignore log lines
	if (bytes.Contains(data, []byte("/")) || bytes.Contains(data, []byte("\\"))) &&
		!bytes.Contains(data, []byte("logger.go")) {
		return types.TypeFile
	}

	// Default to text
	return types.TypeText
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
