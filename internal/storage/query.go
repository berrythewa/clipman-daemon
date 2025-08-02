package storage

import (
    "fmt"
    "time"
    "encoding/json"     // Only needed in this file for JSON operations
    "go.etcd.io/bbolt"  // Must import again
    "go.uber.org/zap"   // Must import again
    "github.com/berrythewa/clipman-daemon/internal/types"
)

// GetLatestContent retrieves the most recent clipboard content (by occurrence)
func (s *BoltStorage) GetLatestContent() (*types.ClipboardContent, error) {
	var latestContent *types.ClipboardContent
	var latestTime time.Time
	var needsHashUpdate bool

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(clipboardBucket))
		return b.ForEach(func(k, v []byte) error {
			var content types.ClipboardContent
			if err := json.Unmarshal(v, &content); err != nil {
				s.logger.Warn("Failed to unmarshal content", zap.Error(err), zap.Binary("hash", k))
				return nil // skip invalid entries
			}
			// Find latest occurrence
			for _, occ := range content.Occurrences {
				if occ.After(latestTime) {
					latestTime = occ
					copyContent := content
					copyContent.Created = occ // Use occurrence time as creation time
					latestContent = &copyContent
				}
			}
			return nil
		})
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get latest content: %w", err)
	}

	// Update database if hash was generated
	if needsHashUpdate && latestContent != nil {
		updateErr := s.db.Update(func(tx *bbolt.Tx) error {
			b := tx.Bucket([]byte(clipboardBucket))
			encoded, err := json.Marshal(latestContent)
			if err != nil {
				return fmt.Errorf("failed to marshal content for hash update: %w", err)
			}
			return b.Put([]byte(latestContent.Hash), encoded)
		})
		if updateErr != nil {
			s.logger.Warn("Failed to update latest content hash", zap.Error(updateErr))
		}
	}

	if latestContent != nil {
		// ALWAYS attempt decompression for backward compatibility
		decompressed, err := compression.DecompressContent(latestContent)
		if err == nil {
			s.logger.Debug("Successfully decompressed latest content",
				zap.String("hash", latestContent.Hash),
				zap.Bool("compressed_flag", latestContent.Compressed))
			latestContent = decompressed
		} else {
			// Decompression failed - try base64 decoding as fallback
			s.logger.Debug("Decompression failed for latest content, trying base64 decode",
				zap.String("hash", latestContent.Hash),
				zap.Error(err))
			latestContent = s.decodeContent(latestContent)
		}
	}

	return latestContent, nil
}

// GetContentSince retrieves content items that have occurred since the specified time
func (s *BoltStorage) GetContentSince(since time.Time) ([]*types.ClipboardContent, error) {
    var results []*types.ClipboardContent

    err := s.db.View(func(tx *bbolt.Tx) error {
        b := tx.Bucket([]byte(clipboardBucket))
        return b.ForEach(func(k, v []byte) error {
            var content types.ClipboardContent
            if err := json.Unmarshal(v, &content); err != nil {
                s.logger.Warn("Failed to unmarshal content", zap.Error(err), zap.Binary("hash", k))
                return nil
            }

            // Only check the latest occurrence (index 0)
            if len(content.Occurrences) > 0 {
                latestOccurrence := content.Occurrences[0] // Newest first
                if latestOccurrence.After(since) || latestOccurrence.Equal(since) {
                    // Handle decompression
                    decompressed, err := compression.DecompressContent(&content)
                    if err == nil {
                        results = append(results, decompressed)
                    } else {
                        decoded := s.decodeContent(&content)
                        results = append(results, decoded)
                    }
                }
            }
            return nil
        })
    })

    return results, err
}

// GetContentByHash retrieves a single clipboard content by its hash
func (s *BoltStorage) GetContentByHash(hash string) (*types.ClipboardContent, error) {
	var content *types.ClipboardContetypes.ClipboardContent
    err := s.db.View(func(tx *bbolt.Tx) error {
        b := tx.Bucket([]byte(clipboardBucket))
        for _, hash := range hashes {
            v := b.Get([]byte(hash))
            if v != nil {
                var content types.ClipboardContent
                if err := json.Unmarshal(v, &content); err == nil {
                    contents = append(contents, &content)
                }
            }
        }
        return nil
    })
    return contents, err
}

// GetContentsByIDs retrieves specific history entries by their IDs
func (s *BoltStorage) GetContentsByIDs(ids []int64) ([]*types.ClipboardContent, error) {
    var contents []*types.ClipboardContent
    idMap := make(map[int64]bool)
    for _, id := range ids {
        idMap[id] = true
    }
    err := s.db.View(func(tx *bbolt.Tx) error {
        b := tx.Bucket([]byte(clipboardBucket))
        return b.ForEach(func(k, v []byte) error {
            var content types.ClipboardContent
            if err := json.Unmarshal(v, &content); err == nil {
                if _, found := idMap[content.Id]; found {
                    contents = append(contents, &content)
                }
            }
            return nil
        })
    })
    return contents, err
}


// GetContentsByHashes retrieves specific history entries by their hashes
func (s *BoltStorage) GetContentsByHashes(hashes []string) ([]*types.ClipboardContent, error) {
    // TODO: Implementation
    return nil, nil
}

// GetContentsByIDs retrieves specific history entries by their IDs
func (s *BoltStorage) GetContentsByIDs(ids []int64) ([]*types.ClipboardContent, error) {
    // TODO: Implementation
    return nil, nil
}

// GetFrequentContent returns the most frequently occurring content
func (s *BoltStorage) GetMostFrequentContent(limit int) ([]*types.ClipboardContent, error) {
	type contentWithFreq struct {
		content *types.ClipboardContent
		freq    int
	}

	var items []contentWithFreq

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(clipboardBucket))
		return b.ForEach(func(k, v []byte) error {
			var content types.ClipboardContent
			if err := json.Unmarshal(v, &content); err != nil {
				s.logger.Warn("Failed to unmarshal content", zap.Error(err), zap.Binary("hash", k))
				return nil // skip invalid entries
			}

			items = append(items, contentWithFreq{
				content: &content,
				freq:    len(content.Occurrences),
			})
			return nil
		})
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get frequent content: %w", err)
	}

	// Sort by frequency descending
	sort.Slice(items, func(i, j int) bool {
		return items[i].freq > items[j].freq
	})

	// Take top N items
	if limit > 0 && limit < len(items) {
		items = items[:limit]
	}

	// Convert to result format
	result := make([]*types.ClipboardContent, len(items))
	for i, item := range items {
		// ALWAYS attempt decompression for backward compatibility
		decompressed, err := compression.DecompressContent(item.content)
		if err == nil {
			s.logger.Debug("Successfully decompressed frequent content",
				zap.String("hash", item.content.Hash),
				zap.Bool("compressed_flag", item.content.Compressed))
			result[i] = decompressed
		} else {
			// Decompression failed - try base64 decoding as fallback
			s.logger.Debug("Decompression failed for frequent content, trying base64 decode",
				zap.String("hash", item.content.Hash),
				zap.Error(err))
			result[i] = s.decodeContent(item.content)
		}
	}

	return result, nil
}

// GetHistory retrieves clipboard history based on the provided options
func (s *BoltStorage) GetHistory(options config.HistoryOptions) ([]*types.ClipboardContent, error) {
	var allContents []*types.ClipboardContent
	var updatedContents []*types.ClipboardContent // Track items that need hash updates

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(clipboardBucket))
		return b.ForEach(func(k, v []byte) error {
			var content types.ClipboardContent
			if err := json.Unmarshal(v, &content); err != nil {
				s.logger.Warn("Failed to unmarshal content", zap.Error(err), zap.Binary("key", k))
				return nil // skip invalid entries
			}

			// BACKWARD COMPATIBILITY: Generate hash if missing
			if content.Hash == "" && len(content.Data) > 0 {
				if len(content.Data) > 1024*1024 { // 1MB threshold
					content.Hash = utils.HashContentBis(content.Data)
				} else {
					content.Hash = utils.HashContent(content.Data)
				}
				s.logger.Debug("Generated missing hash for existing content",
					zap.String("hash", content.Hash),
					zap.String("type", string(content.Type)))
				updatedContents = append(updatedContents, &content)
			}

			allContents = append(allContents, &content)
			return nil
		})
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}

	// Update database records that were missing hashes
	if len(updatedContents) > 0 {
		updateErr := s.db.Update(func(tx *bbolt.Tx) error {
			b := tx.Bucket([]byte(clipboardBucket))
			for _, content := range updatedContents {
				encoded, err := json.Marshal(content)
				if err != nil {
					s.logger.Error("Failed to marshal content for hash update", zap.Error(err))
					continue
				}
				// Use hash as key for new storage format
				if err := b.Put([]byte(content.Hash), encoded); err != nil {
					s.logger.Error("Failed to update content with hash", zap.Error(err))
					continue
				}
			}
			return nil
		})
		if updateErr != nil {
			s.logger.Warn("Failed to update some content hashes", zap.Error(updateErr))
		} else {
			s.logger.Info("Updated missing hashes for existing content", zap.Int("count", len(updatedContents)))
		}
	}

	// Apply filtering to all contents
	var filteredContents []*types.ClipboardContent
	for _, content := range allContents {
		// Apply time filters
		if !options.Since.IsZero() && content.Created.Before(options.Since) {
			continue
		}
		if !options.Before.IsZero() && content.Created.After(options.Before) {
			continue
		}

		// Apply content type filter
		if options.ContentType != "" && content.Type != types.ContentType(options.ContentType) {
			continue
		}

		// Apply size filters
		contentSize := int64(len(content.Data))
		if options.MinSize > 0 && contentSize < options.MinSize {
			continue
		}
		if options.MaxSize > 0 && contentSize > options.MaxSize {
			continue
		}

		// Process content - try decompression first, then base64 if needed
		var processedContent *types.ClipboardContent
		if content.Compressed {
			decompressed, err := compression.DecompressContent(content)
			if err == nil {
				s.logger.Debug("Successfully decompressed content",
					zap.String("hash", content.Hash),
					zap.Bool("compressed_flag", content.Compressed),
					zap.Int("original_size", len(content.Data)),
					zap.Int("decompressed_size", len(decompressed.Data)))
				processedContent = decompressed
			} else {
				s.logger.Debug("Decompression failed, using original content",
					zap.String("hash", content.Hash),
					zap.Error(err))
				processedContent = content
			}
		} else {
			// Try base64 decoding for non-compressed content
			processedContent = s.decodeContent(content)
		}

		filteredContents = append(filteredContents, processedContent)
	}

	// Sort by creation time (newest first by default, unless reverse is specified)
	sort.Slice(filteredContents, func(i, j int) bool {
		if options.Reverse {
			return filteredContents[i].Created.Before(filteredContents[j].Created)
		}
		return filteredContents[i].Created.After(filteredContents[j].Created)
	})

	// Apply limit
	if options.Limit > 0 && int64(len(filteredContents)) > options.Limit {
		filteredContents = filteredContents[:options.Limit]
	}

	return filteredContents, nil
}
