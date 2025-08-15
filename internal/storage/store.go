package storage

import (
	"encoding/json"
	"fmt"
	"time"

	"go.etcd.io/bbolt"
	"go.uber.org/zap"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/berrythewa/clipman-daemon/pkg/utils" // For hashing
	// Add other necessary imports if any
)

// AddContent adds or updates a clipboard item with proper duplicate handling.
// If content with the same hash already exists, it updates the occurrences with a new timestamp.
// Otherwise, it creates a new entry with a unique ID.
func (s *BoltStorage) AddContent(content *types.ClipboardContent) error {
	if content == nil {
		return fmt.Errorf("cannot add nil content")
	}

	err := s.db.Update(func(tx *bbolt.Tx) error {
		clipboardB := tx.Bucket([]byte(clipboardBucket))
		if clipboardB == nil {
			return fmt.Errorf("clipboard bucket '%s' not found", clipboardBucket)
		}
		idIndexB := tx.Bucket([]byte(idIndexBucket))
		if idIndexB == nil {
			return fmt.Errorf("id index bucket '%s' not found", idIndexBucket)
		}

		// --- 1. Ensure content.Hash is set first ---
		if content.Hash == "" {
			// Generate hash based on content size
			if len(content.Data) > 1024*1024 { // 1MB threshold
				content.Hash = utils.HashContentBis(content.Data)
			} else {
				content.Hash = utils.HashContent(content.Data)
			}
			s.logger.Debug("Generated hash for content",
				zap.String("new_hash", content.Hash), zap.String("type", string(content.Type)))
		}

		// --- 2. Check if content with this hash already exists (DUPLICATE DETECTION) ---
		existingContentBytes := clipboardB.Get([]byte(content.Hash))
		if existingContentBytes != nil {
			// Content with this hash already exists - update occurrences
			var existingContent types.ClipboardContent
			if unmarshalErr := json.Unmarshal(existingContentBytes, &existingContent); unmarshalErr != nil {
				s.logger.Error("Failed to unmarshal existing content for duplicate check",
					zap.String("hash", content.Hash), zap.Error(unmarshalErr))
				return fmt.Errorf("failed to unmarshal existing content: %w", unmarshalErr)
			}

			// Add new occurrence timestamp to the existing content
			now := time.Now()
			if existingContent.Occurrences == nil {
				existingContent.Occurrences = []time.Time{now}
			} else {
				// Prepend new occurrence to keep newest at index 0
				existingContent.Occurrences = append([]time.Time{now}, existingContent.Occurrences...)
			}

			// Update the content with new occurrence data
			updatedEncoded, marshalErr := json.Marshal(&existingContent)
			if marshalErr != nil {
				s.logger.Error("Failed to marshal updated content with new occurrence",
					zap.String("hash", content.Hash), zap.Error(marshalErr))
				return fmt.Errorf("failed to marshal updated content: %w", marshalErr)
			}

			// Store the updated content
			if putErr := clipboardB.Put([]byte(content.Hash), updatedEncoded); putErr != nil {
				return fmt.Errorf("failed to update existing content with new occurrence: %w", putErr)
			}

			// Update the input content with the existing ID and updated occurrences for return
			content.Id = existingContent.Id
			content.Occurrences = existingContent.Occurrences
			content.Created = existingContent.Created // Keep original creation time

			s.logger.Debug("Updated existing content with new occurrence",
				zap.String("hash", content.Hash),
				zap.Int64("id", content.Id),
				zap.Int("total_occurrences", len(content.Occurrences)))
			return nil // Early return - duplicate handled
		}

		// --- 3. New content - generate unique ID ---
		if content.Id == 0 {
			// Generate new unique ID for new content
			nextID, err := clipboardB.NextSequence()
			if err != nil {
				s.logger.Error("Failed to generate next sequence ID for new content", zap.Error(err))
				return fmt.Errorf("failed to generate unique ID: %w", err)
			}
			content.Id = int64(nextID)
			s.logger.Debug("Generated new ID for content", zap.Int64("id", content.Id))
		} else {
			// If ID is provided, check if it already exists in the index
			idKey := []byte(fmt.Sprintf("%d", content.Id))
			existingHashBytes := idIndexB.Get(idKey)
			if existingHashBytes != nil {
				s.logger.Warn("Content with provided ID already exists, generating new ID",
					zap.Int64("provided_id", content.Id),
					zap.String("existing_hash", string(existingHashBytes)))
				// Force generation of new ID to avoid conflicts
				nextID, err := clipboardB.NextSequence()
				if err != nil {
					return fmt.Errorf("failed to generate replacement ID: %w", err)
				}
				content.Id = int64(nextID)
			}
		}

		// --- 4. Initialize occurrences for new content ---
		if content.Occurrences == nil {
			content.Occurrences = []time.Time{time.Now()}
		}

		// --- 5. Set creation time if not set ---
		if content.Created.IsZero() {
			content.Created = time.Now()
		}

		// --- 6. Marshal new content for storage ---
		encoded, marshalErr := json.Marshal(content)
		if marshalErr != nil {
			s.logger.Error("Failed to marshal new content for storage",
				zap.String("hash", content.Hash), zap.Error(marshalErr))
			return fmt.Errorf("failed to marshal content for storage: %w", marshalErr)
		}

		// --- 7. Store new content in primary bucket (hash as key) ---
		if putErr := clipboardB.Put([]byte(content.Hash), encoded); putErr != nil {
			return fmt.Errorf("failed to save content to clipboard bucket: %w", putErr)
		}

		// --- 8. Update ID index (ID -> Hash mapping) ---
		idKey := []byte(fmt.Sprintf("%d", content.Id))
		if putErr := idIndexB.Put(idKey, []byte(content.Hash)); putErr != nil {
			return fmt.Errorf("failed to save ID to hash index: %w", putErr)
		}

		s.logger.Debug("Successfully added new content",
			zap.String("hash", content.Hash),
			zap.Int64("id", content.Id),
			zap.String("type", string(content.Type)))
		return nil
	})

	if err != nil {
		s.logger.Error("Failed to add/update content in BoltDB transaction", zap.String("hash", content.Hash), zap.Int64("id", content.Id), zap.Error(err))
		return fmt.Errorf("failed to add/update content: %w", err)
	}

	s.logger.Debug("Content successfully added/updated", zap.String("hash", content.Hash), zap.Int64("id", content.Id), zap.String("type", string(content.Type)))
	return nil
}
