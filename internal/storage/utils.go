package storage

import (
  "encoding/base64"
  "github.com/berrythewa/clipman-daemon/internal/types"
	"go.uber.org/zap"
)


// decodeContentIfNeeded checks if content.Data is base64 encoded and decodes it if it is
func (s *BoltStorage) decodeContent(content *types.ClipboardContent) *types.ClipboardContent {
	if len(content.Data) == 0 {
		return content
	}

	// Try base64 decoding first
	if decoded, err := base64.StdEncoding.DecodeString(string(content.Data)); err == nil {
		// Create a copy of the content with decoded data
		decodedContent := *content
		decodedContent.Data = decoded
		s.logger.Debug("Decoded base64 content",
			zap.String("hash", content.Hash),
			zap.Int("original_size", len(content.Data)),
			zap.Int("decoded_size", len(decoded)))
		return &decodedContent
	}

	// If base64 decoding fails, return original content
	return content
}
