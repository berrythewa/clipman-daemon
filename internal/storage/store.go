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

// AddContent adds or updates a clipboard item.
// It ensures content has a hash and a unique ID, and updates the ID index.
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

		// --- 1. Ensure content.Id is set and unique ---
		if content.Id == 0 { // If ID is not set by the caller (common for new entries)
			// Use BoltDB's NextSequence to generate a unique, monotonically increasing ID
			nextID, err := clipboardB.NextSequence() // Get next sequence from the clipboard bucket
			if err != nil {
				s.logger.Error("Failed to generate next sequence ID for content", zap.Error(err))
				return fmt.Errorf("failed to generate unique ID: %w", err)
			}
			content.Id = int64(nextID)
			s.logger.Debug("Generated new ID for content", zap.Int64("id", content.Id))
		} else {
			// If an ID is provided, ensure it's not a duplicate.
			// This check makes `AddContent` idempotent for existing IDs.
			// If content with this ID already exists, we will retrieve its hash and update.
			// This check becomes more complex if content.Hash can change for the same ID.
			// For simplicity, we assume ID->Hash is a fixed mapping once created.
			idKey := []byte(fmt.Sprintf("%d", content.Id))
			existingHashBytes := idIndexB.Get(idKey)
			if existingHashBytes != nil {
				existingHash := string(existingHashBytes)
				// Content with this ID already exists. Check if hash matches.
				// If hash doesn't match, this is a complex case (ID reuse with different content).
				// For this system, assume ID is tied to a specific hash/content.
				s.logger.Debug("Content with ID already exists, treating as update",
					zap.Int64("id", content.Id),
					zap.String("existing_hash", existingHash))
			}
			// If ID is provided and unique, NextSequence is not advanced by this specific put.
			// It might be better to just rely on NextSequence if you want truly monotonic unique IDs
			// for *all* additions, even if they have an externally provided ID.
			// For this current design, `NextSequence` is only called if `content.Id == 0`.
		}


		// --- 2. Ensure content.Hash is set ---
		if content.Hash == "" {
			// This should ideally be handled before calling AddContent,
			// but providing a fallback. Choose the appropriate hashing algo.
			if len(content.Data) > 1024*1024 { // 1MB threshold
				content.Hash = utils.HashContentBis(content.Data)
			} else {
				content.Hash = utils.HashContent(content.Data)
			}
			s.logger.Warn("Generated missing hash for content before adding",
				zap.String("new_hash", content.Hash), zap.String("type", string(content.Type)))
		}

		// --- 3. Manage Occurrences ---
		if content.Occurrences == nil {
			content.Occurrences = []time.Time{time.Now()}
		} else {
			// Prepend new occurrence to keep newest at Occurrences[0]
			content.Occurrences = append([]time.Time{time.Now()}, content.Occurrences...)
		}

		// --- 4. Marshal content for storage ---
		encoded, marshalErr := json.Marshal(content)
		if marshalErr != nil {
			s.logger.Error("Failed to marshal content for storage", zap.String("hash", content.Hash), zap.Error(marshalErr))
			return fmt.Errorf("failed to marshal content for storage: %w", marshalErr)
		}

		// --- 5. Store content in primary bucket (hash as key) ---
		if putErr := clipboardB.Put([]byte(content.Hash), encoded); putErr != nil {
			return fmt.Errorf("failed to save content to clipboard bucket: %w", putErr)
		}

		// --- 6. Update ID index (ID -> Hash mapping) ---
		idKey := []byte(fmt.Sprintf("%d", content.Id))
		if putErr := idIndexB.Put(idKey, []byte(content.Hash)); putErr != nil {
			// This is critical. If ID index fails, data becomes unretrievable by ID.
			return fmt.Errorf("failed to save ID to hash index: %w", putErr)
		}

		return nil // Transaction successful
	})

	if err != nil {
		s.logger.Error("Failed to add/update content in BoltDB transaction", zap.String("hash", content.Hash), zap.Int64("id", content.Id), zap.Error(err))
		return fmt.Errorf("failed to add/update content: %w", err)
	}

	s.logger.Debug("Content successfully added/updated", zap.String("hash", content.Hash), zap.Int64("id", content.Id), zap.String("type", string(content.Type)))
	return nil
}
