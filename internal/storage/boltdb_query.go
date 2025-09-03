package storage

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.etcd.io/bbolt"
	"go.uber.org/zap"

	"github.com/berrythewa/clipman-daemon/internal/types"
)

// ============================================================================
// BoltDB Query Implementation - All Get* and Query functions
// ============================================================================

// GetContent retrieves a single clipboard content item solely by its ID.
func (s *BoltStorage) GetContent(id int64) (*types.ClipboardContent, error) {
	if id == 0 {
		return nil, fmt.Errorf("ID cannot be zero for GetContent")
	}

	var contentBytes []byte // Will hold the raw JSON bytes of the content
	var retrievedHash string // The hash that will be retrieved from the ID index

	err := s.db.View(func(tx *bbolt.Tx) error {
		// Ensure clipboard bucket exists
		clipboardB := tx.Bucket([]byte(clipboardBucket))
		if clipboardB == nil {
			s.logger.Error("Clipboard bucket not found in DB view for GetContent by ID", zap.String("bucket", clipboardBucket))
			return fmt.Errorf("clipboard bucket '%s' not found", clipboardBucket)
		}

		// Ensure ID index bucket exists for lookup
		idIndexB := tx.Bucket([]byte(idIndexBucket))
		if idIndexB == nil {
			s.logger.Error("ID index bucket not found for GetContent by ID", zap.String("bucket", idIndexBucket))
			return fmt.Errorf("ID index bucket '%s' not found, cannot retrieve content by ID", idIndexBucket)
		}

		// --- 1. Look up the hash using the provided ID from the ID index ---
		idKey := []byte(strconv.FormatInt(id, 10)) // Convert int64 ID to []byte key for BoltDB
		hashFromIndex := idIndexB.Get(idKey)
		if hashFromIndex == nil {
			s.logger.Debug("Hash not found for ID in index", zap.Int64("id", id))
			return fmt.Errorf("content with ID '%d' not found in index", id)
		}
		retrievedHash = string(hashFromIndex) // Convert byte slice to string for use as key

		// --- 2. Retrieve the actual content from the main clipboard bucket using the retrieved hash ---
		v := clipboardB.Get([]byte(retrievedHash)) // Use the hash (converted back to []byte) as the key
		if v == nil {
			s.logger.Error("Content hash found in index but actual content not in main bucket",
				zap.Int64("id", id), zap.String("retrieved_hash", retrievedHash),
				zap.String("details", "data inconsistency: index points to non-existent content"))
			return fmt.Errorf("content with ID '%d' (hash '%s') not found in main storage (data inconsistency)", id, retrievedHash)
		}

		// Make a defensive copy of the value from the database
		contentBytes = make([]byte, len(v))
		copy(contentBytes, v)
		return nil // Success within the transaction
	})

	// Handle errors that occurred during the BoltDB View transaction.
	if err != nil {
		s.logger.Error("Database error during GetContent lookup by ID", zap.Int64("id", id), zap.Error(err))
		return nil, err
	}

	// --- 3. Unmarshal the retrieved bytes into the ClipboardContent struct ---
	var content types.ClipboardContent
	if unmarshalErr := json.Unmarshal(contentBytes, &content); unmarshalErr != nil {
		s.logger.Error("Failed to unmarshal content from DB after ID lookup",
			zap.Int64("id", id), zap.String("hash_used", retrievedHash), zap.Error(unmarshalErr))
		return nil, fmt.Errorf("failed to unmarshal content for ID '%d' (hash '%s'): %w", id, retrievedHash, unmarshalErr)
	}

	// --- 4. Optional: Validate that the ID in the unmarshaled content matches the requested ID ---
	// This is a crucial consistency check to ensure the ID index isn't corrupted or pointing to the wrong data.
	if content.Id != id {
		s.logger.Error("Mismatched ID after retrieving content by ID",
			zap.Int64("id_requested", id),
			zap.Int64("id_in_content", content.Id),
			zap.String("hash_used", retrievedHash),
			zap.String("details", "Content ID does not match requested ID - potential data inconsistency"))
		return nil, fmt.Errorf("data inconsistency: content retrieved by ID '%d' has actual ID '%d'", id, content.Id)
	}

	// --- 5. Process and return the content ---
	s.logger.Debug("Successfully retrieved content by ID", zap.Int64("id", content.Id), zap.String("hash", content.Hash))
	return s.processContent(&content), nil
}

// GetLatestContent retrieves the most recent clipboard content (by occurrence).
// Iterates all entries, which can be inefficient for very large databases.
func (s *BoltStorage) GetLatestContent() (*types.ClipboardContent, error) {
	var latestContent *types.ClipboardContent
	latestTime := time.Time{}

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(clipboardBucket))
		if b == nil {
			s.logger.Warn("Clipboard bucket not found in GetLatestContent.")
			return nil
		}

		return b.ForEach(func(k, v []byte) error {
			var content types.ClipboardContent
			if err := json.Unmarshal(v, &content); err != nil {
				s.logger.Warn("Failed to unmarshal content during GetLatestContent scan", zap.Error(err), zap.Binary("key_bytes", k))
				return nil // Skip invalid entries
			}

			currentContentLatestOcc := time.Time{}
			if len(content.Occurrences) > 0 {
				for _, occ := range content.Occurrences {
					if occ.After(currentContentLatestOcc) {
						currentContentLatestOcc = occ
					}
				}
			} else {
				currentContentLatestOcc = content.Created
			}

			if currentContentLatestOcc.After(latestTime) {
				latestTime = currentContentLatestOcc
				contentCopy := content
				latestContent = &contentCopy
				latestContent.Created = currentContentLatestOcc // Set 'Created' to its latest occurrence
			}

			return nil
		})
	})

	if err != nil {
		s.logger.Error("Failed to iterate content for GetLatestContent", zap.Error(err))
		return nil, fmt.Errorf("failed to get latest content: %w", err)
	}

	if latestContent != nil {
		return s.processContent(latestContent), nil
	}

	s.logger.Debug("No content found in GetLatestContent.")
	return nil, nil
}

// GetContentSince retrieves content items that have occurred since the specified time.
// This function is still present but its functionality is largely superseded by `Query`.
func (s *BoltStorage) GetContentSince(since time.Time) ([]*types.ClipboardContent, error) {
	return s.Query(QueryOptions{
		After: since,
	})
}

// GetContentsByIDs retrieves specific history entries by their IDs.
// Uses the secondary ID -> Hash index for efficient lookups.
func (s *BoltStorage) GetContentsByIDs(ids []int64) ([]*types.ClipboardContent, error) {
	if len(ids) == 0 {
		return []*types.ClipboardContent{}, nil
	}

	hashesToFetch := make([]string, 0, len(ids))

	err := s.db.View(func(tx *bbolt.Tx) error {
		idIndexB := tx.Bucket([]byte(idIndexBucket))
		if idIndexB == nil {
			s.logger.Warn("ID index bucket not found in GetContentsByIDs. Lookups will be inefficient or fail.")
			return fmt.Errorf("ID index bucket '%s' not found", idIndexBucket)
		}

		for _, id := range ids {
			idKey := []byte(fmt.Sprintf("%d", id))
			hashBytes := idIndexB.Get(idKey)
			if hashBytes == nil {
				s.logger.Debug("Hash not found for ID in index", zap.Int64("id", id))
				continue
			}
			hashesToFetch = append(hashesToFetch, string(hashBytes))
		}
		return nil
	})

	if err != nil {
		s.logger.Error("Database error during ID index lookup", zap.Error(err))
		return nil, fmt.Errorf("failed to retrieve hashes by IDs: %w", err)
	}

	// Now fetch the actual content using the hashes found
	return s.GetContentsByHashes(hashesToFetch)
}

// GetContentsByHashes retrieves multiple clipboard contents by their hashes.
func (s *BoltStorage) GetContentsByHashes(hashes []string) ([]*types.ClipboardContent, error) {
	if len(hashes) == 0 {
		return []*types.ClipboardContent{}, nil
	}

	var contents []*types.ClipboardContent
	contentsMap := make(map[string]*types.ClipboardContent) // Use map to avoid duplicates

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(clipboardBucket))
		if b == nil {
			s.logger.Warn("Clipboard bucket not found in GetContentsByHashes.")
			return fmt.Errorf("clipboard bucket '%s' not found", clipboardBucket)
		}

		for _, hash := range hashes {
			v := b.Get([]byte(hash))
			if v == nil {
				s.logger.Debug("Content not found for requested hash", zap.String("hash", hash))
				continue
			}

			var content types.ClipboardContent
			if unmarshalErr := json.Unmarshal(v, &content); unmarshalErr != nil {
				s.logger.Warn("Failed to unmarshal content for hash in GetContentsByHashes",
					zap.String("hash", hash), zap.Error(unmarshalErr))
				continue
			}
			contentsMap[content.Hash] = &content
		}
		return nil
	})

	if err != nil {
		s.logger.Error("Database error during GetContentsByHashes", zap.Error(err))
		return nil, fmt.Errorf("failed to retrieve content by hashes: %w", err)
	}

	for _, content := range contentsMap {
		contents = append(contents, s.processContent(content))
	}

	s.logger.Debug("Retrieved content by hashes", zap.Int("requested_count", len(hashes)), zap.Int("found_count", len(contents)))
	return contents, nil
}

// Query retrieves clipboard history based on the provided options.
// This is the most flexible method for filtering and sorting.
func (s *BoltStorage) Query(options QueryOptions) ([]*types.ClipboardContent, error) {
	return s.executeEnhancedQuery(options)
}

// executeEnhancedQuery performs the enhanced query with all filtering capabilities
func (s *BoltStorage) executeEnhancedQuery(options QueryOptions) ([]*types.ClipboardContent, error) {
	// Handle specific ID/Hash queries first (more efficient)
	if len(options.IDs) > 0 {
		return s.GetContentsByIDs(options.IDs)
	}
	if len(options.Hashes) > 0 {
		return s.GetContentsByHashes(options.Hashes)
	}

	var allContents []*types.ClipboardContent

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(clipboardBucket))
		if b == nil {
			s.logger.Warn("Clipboard bucket not found in Query. Database might be empty or corrupted.")
			return nil
		}

		return b.ForEach(func(k, v []byte) error {
			var content types.ClipboardContent
			if err := json.Unmarshal(v, &content); err != nil {
				s.logger.Warn("Failed to unmarshal content during Query scan", zap.Error(err), zap.Binary("key_bytes", k))
				return nil
			}

			// Add a copy of the content. This content will be filtered and processed later.
			contentCopy := content
			allContents = append(allContents, &contentCopy)
			return nil
		})
	})

	if err != nil {
		s.logger.Error("Failed to iterate content for Query", zap.Error(err))
		return nil, fmt.Errorf("failed to query history: %w", err)
	}

	// Apply all filters
	filteredContents := s.applyFilters(allContents, options)

	// Sort the results
	sortedContents := s.applySorting(filteredContents, options)

	// Apply pagination
	paginatedContents := s.applyPagination(sortedContents, options)

	s.logger.Debug("Query completed",
		zap.Int("total_found", len(allContents)),
		zap.Int("after_filtering", len(filteredContents)),
		zap.Int("final_count", len(paginatedContents)),
		zap.Any("options", options))

	return paginatedContents, nil
}

// applyFilters applies all filtering options to the content list
func (s *BoltStorage) applyFilters(contents []*types.ClipboardContent, options QueryOptions) []*types.ClipboardContent {
	var filtered []*types.ClipboardContent

	for _, content := range contents {
		if s.passesAllFilters(content, options) {
			// Process content (decompress/decode)
			processedContent := s.processContent(content)
			if processedContent != nil {
				filtered = append(filtered, processedContent)
			}
		}
	}

	return filtered
}

// passesAllFilters checks if content passes all specified filters
func (s *BoltStorage) passesAllFilters(content *types.ClipboardContent, options QueryOptions) bool {
	// Time filters
	if !s.passesTimeFilters(content, options) {
		return false
	}

	// Content type filters
	if !s.passesContentTypeFilters(content, options) {
		return false
	}

	// Size filters
	if !s.passesSizeFilters(content, options) {
		return false
	}

	// Device ID filters
	if !s.passesDeviceFilters(content, options) {
		return false
	}

	// Text search filters
	if !s.passesTextFilters(content, options) {
		return false
	}

	// Occurrence filters
	if !s.passesOccurrenceFilters(content, options) {
		return false
	}

	// Compression filters
	if !s.passesCompressionFilters(content, options) {
		return false
	}

	// Tags filters
	if !s.passesTagsFilters(content, options) {
		return false
	}

	return true
}

// passesTimeFilters checks time-based filters
func (s *BoltStorage) passesTimeFilters(content *types.ClipboardContent, options QueryOptions) bool {
	// Handle "Since" as alias for "After"
	after := options.After
	if options.Since.After(after) {
		after = options.Since
	}

	if !after.IsZero() && content.Created.Before(after) {
		return false
	}
	if !options.Before.IsZero() && content.Created.After(options.Before) {
		return false
	}

	return true
}

// passesContentTypeFilters checks content type filters
func (s *BoltStorage) passesContentTypeFilters(content *types.ClipboardContent, options QueryOptions) bool {
	// Single content type filter
	if options.ContentType != "" && content.Type != options.ContentType {
		return false
	}

	// Multiple content types filter
	if len(options.ContentTypes) > 0 {
		found := false
		for _, ct := range options.ContentTypes {
			if content.Type == ct {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// passesSizeFilters checks size-based filters
func (s *BoltStorage) passesSizeFilters(content *types.ClipboardContent, options QueryOptions) bool {
	contentSize := int64(len(content.Data))
	if options.MinSize > 0 && contentSize < options.MinSize {
		return false
	}
	if options.MaxSize > 0 && contentSize > options.MaxSize {
		return false
	}
	return true
}

// passesDeviceFilters checks device ID filters
func (s *BoltStorage) passesDeviceFilters(content *types.ClipboardContent, options QueryOptions) bool {
	if len(options.DeviceIDs) > 0 {
		found := false
		for _, deviceID := range options.DeviceIDs {
			if content.DeviceID == deviceID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// passesTextFilters checks text search filters
func (s *BoltStorage) passesTextFilters(content *types.ClipboardContent, options QueryOptions) bool {
	if options.Contains != "" {
		text := string(content.Data)
		if !options.CaseSensitive {
			text = strings.ToLower(text)
			search := strings.ToLower(options.Contains)
			if !strings.Contains(text, search) {
				return false
			}
		} else {
			if !strings.Contains(text, options.Contains) {
				return false
			}
		}
	}

	// TODO: Add regex support when needed
	if options.Regex != "" {
		// For now, log that regex is not implemented
		s.logger.Debug("Regex filtering not yet implemented", zap.String("pattern", options.Regex))
	}

	return true
}

// passesOccurrenceFilters checks occurrence-based filters
func (s *BoltStorage) passesOccurrenceFilters(content *types.ClipboardContent, options QueryOptions) bool {
	occurrenceCount := len(content.Occurrences)

	if options.HasOccurrences != nil {
		hasOcc := occurrenceCount > 0
		if *options.HasOccurrences != hasOcc {
			return false
		}
	}

	if options.MinOccurrences > 0 && occurrenceCount < options.MinOccurrences {
		return false
	}

	if options.MaxOccurrences > 0 && occurrenceCount > options.MaxOccurrences {
		return false
	}

	return true
}

// passesCompressionFilters checks compression status filters
func (s *BoltStorage) passesCompressionFilters(content *types.ClipboardContent, options QueryOptions) bool {
	if options.CompressedOnly != nil {
		if *options.CompressedOnly != content.Compressed {
			return false
		}
	}
	return true
}

// passesTagsFilters checks if content matches tag filters
func (s *BoltStorage) passesTagsFilters(content *types.ClipboardContent, options QueryOptions) bool {
	// If no tags filter is specified, pass all content
	if len(options.Tags) == 0 {
		return true
	}

	// Check if content has ALL the specified tags (AND logic)
	for _, requiredTag := range options.Tags {
		found := false
		for _, contentTag := range content.Tags {
			if contentTag == requiredTag {
				found = true
				break
			}
		}
		if !found {
			return false // Content doesn't have this required tag
		}
	}

	return true // Content has all required tags
}

// applySorting sorts the content based on the specified options
func (s *BoltStorage) applySorting(contents []*types.ClipboardContent, options QueryOptions) []*types.ClipboardContent {
	if len(contents) <= 1 {
		return contents
	}

	sort.Slice(contents, func(i, j int) bool {
		return s.compareContent(contents[i], contents[j], options)
	})

	return contents
}

// compareContent compares two content items based on sorting options
func (s *BoltStorage) compareContent(a, b *types.ClipboardContent, options QueryOptions) bool {
	var result bool

	switch options.SortBy {
	case SortByCreated, "":
		result = a.Created.After(b.Created) // Default: newest first

	case SortByLastSeen:
		// Find latest occurrence for each
		latestA := a.Created
		for _, occ := range a.Occurrences {
			if occ.After(latestA) {
				latestA = occ
			}
		}

		latestB := b.Created
		for _, occ := range b.Occurrences {
			if occ.After(latestB) {
				latestB = occ
			}
		}

		result = latestA.After(latestB)

	case SortBySize:
		result = len(a.Data) > len(b.Data) // Larger first

	case SortByType:
		result = string(a.Type) < string(b.Type) // Alphabetical

	case SortById:
		result = a.Id > b.Id // Higher ID first

	case SortByOccurrence:
		result = len(a.Occurrences) > len(b.Occurrences) // More occurrences first

	default:
		result = a.Created.After(b.Created) // Fallback to creation time
	}

	// Apply sort order
	if options.SortOrder == SortAsc {
		result = !result
	}

	return result
}

// applyPagination applies limit and offset to the content list
func (s *BoltStorage) applyPagination(contents []*types.ClipboardContent, options QueryOptions) []*types.ClipboardContent {
	if len(contents) == 0 {
		return contents
	}

	// Apply offset
	start := int(options.Offset)
	if start >= len(contents) {
		return []*types.ClipboardContent{}
	}
	if start > 0 {
		contents = contents[start:]
	}

	// Apply limit
	if options.Limit > 0 && int64(len(contents)) > options.Limit {
		contents = contents[:options.Limit]
	}

	return contents
}
