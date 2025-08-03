package storage

import (
	"encoding/json"
	"fmt"
	"time"

	"go.etcd.io/bbolt"
	"go.uber.org/zap"

	"github.com/berrythewa/clipman-daemon/internal/types"
)

// DeleteOptions defines criteria for deleting clipboard content in a batch.
// This allows flexible deletion based on various time granularities.
type DeleteOptions struct {
	// Delete all content with a creation/occurrence time before this.
	// E.g., if set to 2023-01-01 00:00:00, deletes everything created/occurred on 2022 and earlier.
	Before time.Time

	// Delete all content with a creation/occurrence time after this.
	// E.g., if set to 2023-12-31 23:59:59, deletes everything created/occurred on 2024 and later.
	After time.Time

	// Only consider content created within this specific year.
	// If set (e.g., 2023), `Before` and `After` dates will be constrained to this year.
	Year int

	// Only consider content created within this specific month (1-12).
	// If set (e.g., 1 for January), `Before` and `After` dates will be constrained to this month.
	Month time.Month

	// Only consider content created within this specific day of the month (1-31).
	// If set (e.g., 15), `Before` and `After` dates will be constrained to this day.
	Day int

	// Optional: Delete only specific content types. If empty, all types are considered.
	ContentType types.ContentType
}


// DeleteContent deletes a single clipboard content item by its hash.
func (s *BoltStorage) DeleteContent(hash string) error {
	if hash == "" {
		return fmt.Errorf("hash cannot be empty")
	}

	err := s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(clipboardBucket))
		if b == nil {
			s.logger.Warn("Clipboard bucket not found during DeleteContent.", zap.String("hash", hash))
			return fmt.Errorf("clipboard bucket '%s' not found", clipboardBucket)
		}

		// First, retrieve the content to get its ID for index deletion
		v := b.Get([]byte(hash))
		if v == nil {
			s.logger.Debug("Content with hash not found for deletion", zap.String("hash", hash))
			return fmt.Errorf("content with hash '%s' not found for deletion", hash)
		}

		var content types.ClipboardContent
		if unmarshalErr := json.Unmarshal(v, &content); unmarshalErr != nil {
			s.logger.Error("Failed to unmarshal content for deletion (to get ID)", zap.String("hash", hash), zap.Error(unmarshalErr))
			// Decide if you want to abort or proceed with hash deletion only
			return fmt.Errorf("failed to unmarshal content for hash '%s' during delete: %w", hash, unmarshalErr)
		}

		// Delete from main clipboard bucket
		if delErr := b.Delete([]byte(hash)); delErr != nil {
			return fmt.Errorf("failed to delete content from clipboard bucket: %w", delErr)
		}

		// Delete from ID index bucket
		idIndexB := tx.Bucket([]byte(idIndexBucket))
		if idIndexB == nil {
			s.logger.Warn("ID index bucket not found during DeleteContent.", zap.String("hash", hash))
			// Not a critical error if id_index is missing but main content deleted
		} else {
			idKey := []byte(fmt.Sprintf("%d", content.Id))
			if delErr := idIndexB.Delete(idKey); delErr != nil {
				s.logger.Warn("Failed to delete content from ID index bucket", zap.String("hash", hash), zap.Int64("id", content.Id), zap.Error(delErr))
				// Log but don't return error, as main content is already deleted
			}
		}
		return nil
	})

	if err != nil {
		s.logger.Error("Failed to delete content by hash", zap.String("hash", hash), zap.Error(err))
		return fmt.Errorf("failed to delete content by hash '%s': %w", hash, err)
	}

	s.logger.Info("Content successfully deleted by hash", zap.String("hash", hash))
	return nil
}

// DeleteContentsByIDs deletes multiple clipboard content items by their IDs.
// Returns the count of successfully deleted items.
func (s *BoltStorage) DeleteContentsByIDs(ids []int64) (int, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	deletedCount := 0
	err := s.db.Update(func(tx *bbolt.Tx) error {
		clipboardB := tx.Bucket([]byte(clipboardBucket))
		idIndexB := tx.Bucket([]byte(idIndexBucket))

		if clipboardB == nil {
			s.logger.Warn("Clipboard bucket not found during DeleteContentsByIDs.")
			return fmt.Errorf("clipboard bucket '%s' not found", clipboardBucket)
		}
		if idIndexB == nil {
			s.logger.Warn("ID index bucket not found during DeleteContentsByIDs.")
			// Continue, but log as we cannot maintain index consistency
		}

		for _, id := range ids {
			idKey := []byte(fmt.Sprintf("%d", id))
			var hashBytes []byte = nil

			if idIndexB != nil {
				hashBytes = idIndexB.Get(idKey) // Get hash from ID index
				if hashBytes == nil {
					s.logger.Debug("Hash not found for ID in index, skipping ID deletion", zap.Int64("id", id))
					continue // This ID is not in the index, or already deleted
				}
			} else {
				// Fallback if ID index is missing: needs a full scan to find hash by ID.
				// This is very inefficient and should ideally not happen if idIndexB exists.
				s.logger.Warn("Attempting full scan to find hash for ID because ID index is missing or corrupted.", zap.Int64("id", id))
				foundHash := ""
				scanErr := clipboardB.ForEach(func(k, v []byte) error {
					var content types.ClipboardContent
					if err := json.Unmarshal(v, &content); err != nil {
						return nil // Skip invalid
					}
					if content.Id == id {
						foundHash = string(k) // Assuming k is the hash
						return fmt.Errorf("found") // Special error to stop ForEach
					}
					return nil
				})
				if scanErr != nil && scanErr.Error() == "found" {
					hashBytes = []byte(foundHash)
				} else {
					s.logger.Warn("Failed to find hash for ID via full scan", zap.Int64("id", id), zap.Error(scanErr))
					continue
				}
			}

			if hashBytes == nil || len(hashBytes) == 0 {
				s.logger.Debug("No hash found for ID (after index or scan attempt)", zap.Int64("id", id))
				continue
			}
			hash := string(hashBytes)

			// Delete from main clipboard bucket
			if delErr := clipboardB.Delete([]byte(hash)); delErr != nil {
				s.logger.Error("Failed to delete content from clipboard bucket by ID's hash",
					zap.Int64("id", id), zap.String("hash", hash), zap.Error(delErr))
				return fmt.Errorf("failed to delete content for ID %d (hash %s): %w", id, hash, delErr)
			}
			s.logger.Debug("Deleted content from clipboard bucket by ID's hash", zap.Int64("id", id), zap.String("hash", hash))

			// Delete from ID index bucket
			if idIndexB != nil {
				if delErr := idIndexB.Delete(idKey); delErr != nil {
					s.logger.Warn("Failed to delete ID from ID index bucket",
						zap.Int64("id", id), zap.String("hash", hash), zap.Error(delErr))
					// Continue, as main content is already deleted
				} else {
					s.logger.Debug("Deleted ID from ID index bucket", zap.Int64("id", id))
				}
			}
			deletedCount++
		}
		return nil
	})

	if err != nil {
		s.logger.Error("Failed to delete contents by IDs", zap.Error(err))
		return deletedCount, fmt.Errorf("failed to delete contents by IDs: %w", err)
	}

	s.logger.Info("Successfully deleted contents by IDs", zap.Int("count", deletedCount))
	return deletedCount, nil
}

// DeleteContentsByHashes deletes multiple clipboard content items by their hashes.
// Returns the count of successfully deleted items.
func (s *BoltStorage) DeleteContentsByHashes(hashes []string) (int, error) {
	if len(hashes) == 0 {
		return 0, nil
	}

	deletedCount := 0
	err := s.db.Update(func(tx *bbolt.Tx) error {
		clipboardB := tx.Bucket([]byte(clipboardBucket))
		idIndexB := tx.Bucket([]byte(idIndexBucket)) // Get a reference to the ID index bucket

		if clipboardB == nil {
			s.logger.Warn("Clipboard bucket not found during DeleteContentsByHashes.")
			return fmt.Errorf("clipboard bucket '%s' not found", clipboardBucket)
		}

		for _, hash := range hashes {
			// Retrieve the content first to get its ID (if ID index exists)
			var contentID int64
			v := clipboardB.Get([]byte(hash))
			if v != nil {
				var content types.ClipboardContent
				if unmarshalErr := json.Unmarshal(v, &content); unmarshalErr == nil {
					contentID = content.Id
				} else {
					s.logger.Warn("Failed to unmarshal content to get ID for index deletion",
						zap.String("hash", hash), zap.Error(unmarshalErr))
					// Proceed with deleting main content, but won't be able to delete from index by ID.
				}
			} else {
				s.logger.Debug("Content with hash not found for deletion, skipping", zap.String("hash", hash))
				continue // Content not found, nothing to delete
			}

			// Delete from main clipboard bucket
			if delErr := clipboardB.Delete([]byte(hash)); delErr != nil {
				s.logger.Error("Failed to delete content from clipboard bucket",
					zap.String("hash", hash), zap.Error(delErr))
				return fmt.Errorf("failed to delete content with hash %s: %w", hash, delErr)
			}
			s.logger.Debug("Deleted content from clipboard bucket", zap.String("hash", hash))


			// Delete from ID index bucket if we got an ID and the bucket exists
			if contentID != 0 && idIndexB != nil {
				idKey := []byte(fmt.Sprintf("%d", contentID))
				if delErr := idIndexB.Delete(idKey); delErr != nil {
					s.logger.Warn("Failed to delete content from ID index bucket",
						zap.String("hash", hash), zap.Int64("id", contentID), zap.Error(delErr))
					// Log but continue, as main content is deleted
				} else {
					s.logger.Debug("Deleted ID from ID index bucket", zap.Int64("id", contentID))
				}
			}
			deletedCount++
		}
		return nil
	})

	if err != nil {
		s.logger.Error("Failed to delete contents by hashes", zap.Error(err))
		return deletedCount, fmt.Errorf("failed to delete contents by hashes: %w", err)
	}

	s.logger.Info("Successfully deleted contents by hashes", zap.Int("count", deletedCount))
	return deletedCount, nil
}


// DeleteByTimestamp deletes clipboard content based on time criteria.
// Returns the count of successfully deleted items.
func (s *BoltStorage) DeleteByTimestamp(options DeleteOptions) (int, error) {
	// First, identify all content items that match the deletion criteria
	// We need to do this in a View transaction or separately to collect keys to delete.
	var hashesToDelete []string
	var idsToDelete []int64 // Keep track of IDs for index deletion

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(clipboardBucket))
		if b == nil {
			s.logger.Warn("Clipboard bucket not found during DeleteByTimestamp identification phase.")
			return nil
		}

		return b.ForEach(func(k, v []byte) error {
			var content types.ClipboardContent
			if err := json.Unmarshal(v, &content); err != nil {
				s.logger.Warn("Failed to unmarshal content during DeleteByTimestamp scan", zap.Error(err), zap.Binary("key_bytes", k))
				return nil // Skip invalid entries
			}

			// Determine the effective creation/occurrence time for comparison.
			// Prioritize latest occurrence, fallback to Created.
			compareTime := content.Created
			if len(content.Occurrences) > 0 {
				latestOcc := content.Occurrences[0] // Assuming newest at index 0
				for _, occ := range content.Occurrences { // If not guaranteed, iterate
					if occ.After(latestOcc) {
						latestOcc = occ
					}
				}
				compareTime = latestOcc
			}


			// Apply Year filter
			if options.Year != 0 && compareTime.Year() != options.Year {
				return nil
			}
			// Apply Month filter
			if options.Month != 0 && compareTime.Month() != options.Month {
				return nil
			}
			// Apply Day filter
			if options.Day != 0 && compareTime.Day() != options.Day {
				return nil
			}

			// Apply Before filter
			if !options.Before.IsZero() && !compareTime.Before(options.Before) {
				return nil // Don't delete if not before the specified time
			}
			// Apply After filter
			if !options.After.IsZero() && !compareTime.After(options.After) {
				return nil // Don't delete if not after the specified time
			}

			// Apply ContentType filter
			if options.ContentType != "" && content.Type != options.ContentType {
				return nil
			}

			// If all filters pass, mark for deletion
			hashesToDelete = append(hashesToDelete, string(k))
			idsToDelete = append(idsToDelete, content.Id)
			return nil
		})
	})

	if err != nil {
		s.logger.Error("Failed to identify content for deletion by timestamp", zap.Any("options", options), zap.Error(err))
		return 0, fmt.Errorf("failed to identify content for deletion: %w", err)
	}

	if len(hashesToDelete) == 0 {
		s.logger.Info("No content found matching deletion criteria.", zap.Any("options", options))
		return 0, nil
	}

	// Now perform the actual deletions in a separate Update transaction
	deletedCount := 0
	err = s.db.Update(func(tx *bbolt.Tx) error {
		clipboardB := tx.Bucket([]byte(clipboardBucket))
		idIndexB := tx.Bucket([]byte(idIndexBucket))

		if clipboardB == nil {
			return fmt.Errorf("clipboard bucket '%s' not found during deletion", clipboardBucket)
		}

		for i, hash := range hashesToDelete {
			// Delete from main clipboard bucket
			if delErr := clipboardB.Delete([]byte(hash)); delErr != nil {
				s.logger.Warn("Failed to delete content from clipboard bucket during batch delete",
					zap.String("hash", hash), zap.Error(delErr))
				// Continue with other deletions even if one fails
			} else {
				deletedCount++
				s.logger.Debug("Deleted content from clipboard bucket in batch", zap.String("hash", hash))

				// Delete from ID index bucket if it exists and we have a valid ID
				if idIndexB != nil && i < len(idsToDelete) { // Check bounds for idsToDelete
					id := idsToDelete[i]
					if id != 0 { // Ensure ID is not zero (default for uninitialized int64)
						idKey := []byte(fmt.Sprintf("%d", id))
						if delErr := idIndexB.Delete(idKey); delErr != nil {
							s.logger.Warn("Failed to delete ID from ID index bucket during batch delete",
								zap.String("hash", hash), zap.Int64("id", id), zap.Error(delErr))
							// Log but continue
						} else {
							s.logger.Debug("Deleted ID from ID index bucket in batch", zap.Int64("id", id))
						}
					}
				}
			}
		}
		return nil
	})

	if err != nil {
		s.logger.Error("Failed to execute batch deletion by timestamp", zap.Any("options", options), zap.Error(err))
		return deletedCount, fmt.Errorf("failed to execute batch deletion by timestamp: %w", err)
	}

	s.logger.Info("Successfully deleted content by timestamp criteria",
		zap.Int("deleted_count", deletedCount), zap.Any("options", options))
	return deletedCount, nil
}

// DeleteAllContent empties the entire clipboard history and its ID index.
// Use with extreme caution!
func (s *BoltStorage) DeleteAllContent() error {
	s.logger.Warn("Initiating deletion of ALL clipboard content! This action is irreversible.")
	err := s.db.Update(func(tx *bbolt.Tx) error {
		// Recreate the clipboard bucket, which effectively deletes all contents
		if err := tx.DeleteBucket([]byte(clipboardBucket)); err != nil && err != bbolt.ErrBucketNotFound {
			return fmt.Errorf("failed to delete clipboard bucket: %w", err)
		}
		if _, err := tx.CreateBucket([]byte(clipboardBucket)); err != nil {
			return fmt.Errorf("failed to recreate clipboard bucket: %w", err)
		}

		// Recreate the ID index bucket
		if err := tx.DeleteBucket([]byte(idIndexBucket)); err != nil && err != bbolt.ErrBucketNotFound {
			return fmt.Errorf("failed to delete ID index bucket: %w", err)
		}
		if _, err := tx.CreateBucket([]byte(idIndexBucket)); err != nil {
			return fmt.Errorf("failed to recreate ID index bucket: %w", err)
		}

		return nil
	})

	if err != nil {
		s.logger.Error("Failed to delete all content", zap.Error(err))
		return fmt.Errorf("failed to delete all content: %w", err)
	}

	s.logger.Info("All clipboard content successfully deleted.")
	return nil
}

// CountContent returns the total number of content items in the storage.
func (s *BoltStorage) CountContent() (int, error) {
	count := 0
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(clipboardBucket))
		if b == nil {
			return nil // Bucket not found means 0 items
		}
		stats := b.Stats()
		count = stats.KeyN
		return nil
	})
	if err != nil {
		s.logger.Error("Failed to count content", zap.Error(err))
		return 0, fmt.Errorf("failed to count content: %w", err)
	}
	s.logger.Debug("Content count", zap.Int("count", count))
	return count, nil
}
