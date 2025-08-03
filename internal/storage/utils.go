package storage

import (
  "encoding/base64"
  "github.com/berrythewa/clipman-daemon/internal/types"
	"go.uber.org/zap"
  "github.com/berrythewa/clipman-daemon/pkg/compression"

)


// decodeContentIfNeeded checks if content.Data is base64 encoded and decodes it if it is
// func (s *BoltStorage) decodeContent(content *types.ClipboardContent) *types.ClipboardContent {
// 	if len(content.Data) == 0 {
// 		return content
// 	}
//
// 	// Try base64 decoding first
// 	if decoded, err := base64.StdEncoding.DecodeString(string(content.Data)); err == nil {
// 		// Create a copy of the content with decoded data
// 		decodedContent := *content
// 		decodedContent.Data = decoded
// 		s.logger.Debug("Decoded base64 content",
// 			zap.String("hash", content.Hash),
// 			zap.Int("original_size", len(content.Data)),
// 			zap.Int("decoded_size", len(decoded)))
// 		return &decodedContent
// 	}
//
// 	// If base64 decoding fails, return original content
// 	return content
// }

// processContent applies decompression and base64 decoding logic to a content item.
// It returns a new ClipboardContent pointer to avoid modifying the original during processing.
func (s *BoltStorage) processContent(content *types.ClipboardContent) *types.ClipboardContent {
	if content == nil {
		return nil
	}

	// Create a copy of the *value* to avoid modifying the original struct
	// if it was obtained directly from `Unmarshal` inside a `ForEach` loop.
	contentCopy := *content
	processedContent := &contentCopy

	if processedContent.Compressed {
		decompressed, err := compression.DecompressContent(processedContent)
		if err == nil {
			s.logger.Debug("Successfully decompressed content",
				zap.String("hash", processedContent.Hash),
				zap.Bool("compressed_flag", processedContent.Compressed),
				zap.Int("original_size", len(content.Data)),
				zap.Int("decompressed_size", len(decompressed.Data)))
			processedContent = decompressed
		} else {
			s.logger.Warn("Decompression failed for content, attempting base64 decode as fallback",
				zap.String("hash", processedContent.Hash),
				zap.Error(err))
			// If decompression fails, try base64 decode on the *original* content (not the partially processed one)
			processedContent = s.decodeContent(content)
		}
	} else {
		// If not compressed, always try base64 decode for older entries or specific types
		processedContent = s.decodeContent(content)
	}

	return processedContent
}

// decodeContent attempts to base64 decode content.Data if it's not compressed.
// This handles older entries or those that are text/plain that might have been
// base64 encoded for storage, but not marked as 'compressed'.
// It returns a new ClipboardContent pointer to avoid modifying the original.
func (s *BoltStorage) decodeContent(content *types.ClipboardContent) *types.ClipboardContent {
	if content == nil {
		return nil
	}
	// Create a copy to avoid modifying the original struct
	decodedContent := *content

	if !decodedContent.Compressed && len(decodedContent.Data) > 0 {
		decodedData, err := base64.StdEncoding.DecodeString(string(decodedContent.Data))
		if err == nil {
			decodedContent.Data = decodedData
			s.logger.Debug("Successfully base64 decoded content", zap.String("hash", decodedContent.Hash))
			return &decodedContent
		} else {
			s.logger.Debug("Base64 decode failed for content, using original data",
				zap.String("hash", decodedContent.Hash),
				zap.Error(err))
			return content // Fallback to original data if base64 decoding fails
		}
	}
	return content // Return original if already compressed or no data to decode
}
