package storage

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"go.etcd.io/bbolt"
	"go.uber.org/zap"

	"github.com/berrythewa/clipman-daemon/internal/types"
)

const (
	clipboardBucket = "clipboard"
	// Optional: Use a separate bucket for ID to Hash mapping if ID lookups are critical and frequent
	idIndexBucket = "id_index"
)

// Record represents the raw data stored in BoltDB.
// This allows for internal management (like storing a hash that might be missing
// in old ClipboardContent data) without exposing these internal details.
// It maps closely to types.ClipboardContent but is the "on-disk" representation.
type Record struct {
	types.ClipboardContent // Embeds the public struct, inheriting its fields
	// Add any internal-only fields here if needed for storage
	// For example, if you stored the original key or format version.
}

// StorageConfig holds configuration for BoltStorage.
type StorageConfig struct {
	DBPath   string
	Logger   *zap.Logger
	DeviceID string
}

// QueryOptions defines criteria for querying clipboard content.
// This replaces CLI-specific options and makes the API more flexible.
type QueryOptions struct {
	Before      time.Time   // Retrieve content created before this time
	After       time.Time   // Retrieve content created after this time
	ContentType types.ContentType // Filter by content type (e.g., "text", "image")
	MinSize     int64       // Minimum data size in bytes
	MaxSize     int64       // Maximum data size in bytes
	Limit       int64       // Maximum number of results to return
	Reverse     bool        // Sort by oldest first (default is newest first)
	// Add specific ID or Hash filters here if they become complex,
	// but for single/multiple specific IDs/hashes, dedicated methods are often clearer.
}

// IStorage defines the interface for clipboard content storage operations.
// This makes the storage layer testable and interchangeable.
type IStorage interface {
	AddContent(content *types.ClipboardContent) error
	GetContent(hash string) (*types.ClipboardContent, error) // Renamed from getContentByHash
	GetLatestContent() (*types.ClipboardContent, error)
	GetContentSince(since time.Time) ([]*types.ClipboardContent, error) // Deprecated by Query(options)? Or keep for simple cases.
	GetContentsByIDs(ids []int64) ([]*types.ClipboardContent, error)
	GetContentsByHashes(hashes []string) ([]*types.ClipboardContent, error)
	Query(options QueryOptions) ([]*types.ClipboardContent, error) // New flexible query method
	Close() error
	// Add other methods like DeleteContent, Count, etc.
}

// BoltStorage implements the IStorage interface using BoltDB.
type BoltStorage struct {
	db       *bbolt.DB
	logger   *zap.Logger
	deviceID string
}

// NewBoltStorage initializes a new BoltStorage and implements IStorage.
func NewBoltStorage(config StorageConfig) (IStorage, error) {
	db, err := bbolt.Open(config.DBPath, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open bolt database: %w", err)
	}

	if err := db.Update(func(tx *bbolt.Tx) error {
		// Create primary clipboard bucket
		_, err := tx.CreateBucketIfNotExists([]byte(clipboardBucket))
		if err != nil {
			return err
		}
		// Create ID index bucket (optional, for efficient ID lookups)
		_, err = tx.CreateBucketIfNotExists([]byte(idIndexBucket))
		return err
	}); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create storage buckets: %w", err)
	}

	storage := &BoltStorage{
		db:       db,
		logger:   config.Logger,
		deviceID: config.DeviceID,
	}

	config.Logger.Debug("BoltStorage initialized", zap.String("db_path", config.DBPath), zap.String("device_id", config.DeviceID))
	return storage, nil // Return the interface type
}

// Close ends database operations.
func (s *BoltStorage) Close() error {
	s.logger.Debug("Closing BoltStorage database.")
	return s.db.Close()
}


// ============================================================================
// IStorage Implementations
// ============================================================================

// GetContent retrieves single clipboard content by its hash.
func (s *BoltStorage) GetContent(hash string) (*types.ClipboardContent, error) {
	var contentBytes []byte
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(clipboardBucket))
		if b == nil {
			s.logger.Error("Clipboard bucket not found in DB view for GetContent", zap.String("bucket", clipboardBucket))
			return fmt.Errorf("clipboard bucket '%s' not found", clipboardBucket)
		}

		v := b.Get([]byte(hash))
		if v == nil {
			s.logger.Debug("Content not found for hash", zap.String("hash", hash))
			return fmt.Errorf("content with hash '%s' not found", hash)
		}

		contentBytes = make([]byte, len(v))
		copy(contentBytes, v)
		return nil
	})

	if err != nil {
		s.logger.Error("Database error during GetContent", zap.String("hash", hash), zap.Error(err))
		return nil, err
	}

	var content types.ClipboardContent
	if unmarshalErr := json.Unmarshal(contentBytes, &content); unmarshalErr != nil {
		s.logger.Error("Failed to unmarshal content from DB", zap.String("hash", hash), zap.Error(unmarshalErr))
		return nil, fmt.Errorf("failed to unmarshal content for hash '%s': %w", hash, unmarshalErr)
	}

	s.logger.Debug("Successfully retrieved raw content by hash", zap.String("hash", hash))
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

	// Apply filtering and processing in memory
	var filteredAndProcessedContents []*types.ClipboardContent
	for _, content := range allContents {
		// Apply time filters based on content.Created or latest occurrence
		// (Using Created for consistency with filter options)
		if !options.After.IsZero() && content.Created.Before(options.After) {
			continue
		}
		if !options.Before.IsZero() && content.Created.After(options.Before) {
			continue
		}

		// Apply content type filter
		if options.ContentType != "" && content.Type != options.ContentType {
			continue
		}

		// Apply size filters (based on raw data size for efficiency, before decompression)
		contentSize := int64(len(content.Data))
		if options.MinSize > 0 && contentSize < options.MinSize {
			continue
		}
		if options.MaxSize > 0 && contentSize > options.MaxSize {
			continue
		}

		// Process content (decompress/decode)
		processedContent := s.processContent(content)
		if processedContent != nil {
			filteredAndProcessedContents = append(filteredAndProcessedContents, processedContent)
		}
	}

	// Sort by creation time
	sort.Slice(filteredAndProcessedContents, func(i, j int) bool {
		if options.Reverse { // Oldest first
			return filteredAndProcessedContents[i].Created.Before(filteredAndProcessedContents[j].Created)
		}
		return filteredAndProcessedContents[i].Created.After(filteredAndProcessedContents[j].Created) // Newest first (default)
	})

	// Apply limit
	if options.Limit > 0 && int64(len(filteredAndProcessedContents)) > options.Limit {
		filteredAndProcessedContents = filteredAndProcessedContents[:options.Limit]
	}

	s.logger.Debug("Query completed filtering and processing", zap.Int("total_found", len(allContents)), zap.Int("filtered_count", len(filteredAndProcessedContents)), zap.Any("options", options))
	return filteredAndProcessedContents, nil
}
