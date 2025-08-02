package storage


import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/berrythewa/clipman-daemon/pkg/compression"
	"github.com/berrythewa/clipman-daemon/pkg/utils"

	"go.etcd.io/bbolt"
	"go.uber.org/zap"
)

// SaveContent saves a clipboard content item to the database with hash-based deduplication and occurrence tracking
func (s *BoltStorage) SaveContent(content *types.ClipboardContent) error {
	now := time.Now()
	// Generate hash based on content size
	if len(content.Data) > 1024*1024 { // 1MB threshold
		content.Hash = utils.HashContentBis(content.Data) // Use faster hash for large content
	} else {
		content.Hash = utils.HashContent(content.Data) // Use SHA-256 for normal content
	}

	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(clipboardBucket))
		// Use hash as the key
		v := b.Get([]byte(content.Hash))
		if v != nil {
			// Existing entry: update occurrences
			var existing types.ClipboardContent
			if err := json.Unmarshal(v, &existing); err == nil {
				// Add new occurrence
				existing.Occurrences = append(existing.Occurrences, now)
				// Sort occurrences in descending order (newest first)
				sort.Slice(existing.Occurrences, func(i, j int) bool {
					return existing.Occurrences[i].After(existing.Occurrences[j])
				})

				// Update metadata
				existing.Created = existing.Occurrences[0] // Most recent occurrence

				s.logger.Debug("Updated content occurrences",
					zap.String("hash", existing.Hash),
					zap.Int("occurrence_count", len(existing.Occurrences)),
					zap.Time("latest", existing.Created))

				encoded, err := json.Marshal(existing)
				if err != nil {
					return fmt.Errorf("failed to marshal updated content: %w", err)
				}
				return b.Put([]byte(content.Hash), encoded)
			}
		}
		// New entry
		id, err := b.NextSequence()
		if err != nil {
			return fmt.Errorf("failed to get next sequence ID: %w", err)
		}

		content.Created = now
		content.Occurrences = []time.Time{now}
		content.Id = int64(id)
		s.logger.Debug("New content added",
			zap.Int64("id", content.Id),
			zap.String("hash", content.Hash),
			zap.Time("created", content.Created),
			zap.String("type", string(content.Type)))

		encoded, err := json.Marshal(content)
		if err != nil {
			return fmt.Errorf("failed to marshal new content: %w", err)
		}
		return b.Put([]byte(content.Hash), encoded)
	})
}

// UpdateContent updates existing content by hash, preserving occurrences
func (s *BoltStorage) UpdateContent(hash string, newData []byte, newType types.ContentType) error {
    return s.db.Update(func(tx *bbolt.Tx) error {
        b := tx.Bucket([]byte(clipboardBucket))

        // Get existing content
        v := b.Get([]byte(hash))
        if v == nil {
            return fmt.Errorf("content with hash %s not found", hash)
        }

        var existing types.ClipboardContent
        if err := json.Unmarshal(v, &existing); err != nil {
            return fmt.Errorf("failed to unmarshal existing content: %w", err)
        }

        // Update the content but preserve metadata
        existing.Data = newData
        existing.Type = newType
        // Generate new hash for the updated content
        if len(newData) > 1024*1024 {
            existing.Hash = utils.HashContentBis(newData)
        } else {
            existing.Hash = utils.HashContent(newData)
        }

        // Delete old entry and save with new hash
        if err := b.Delete([]byte(hash)); err != nil {
            return err
        }

        encoded, err := json.Marshal(existing)
        if err != nil {
            return fmt.Errorf("failed to marshal updated content: %w", err)
        }

        return b.Put([]byte(existing.Hash), encoded)
    })
}
