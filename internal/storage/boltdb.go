package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/berrythewa/clipman-daemon/pkg/compression"

	"go.etcd.io/bbolt"
	"go.uber.org/zap"
)

const (
	clipboardBucket = "clipboard"
	defaultMaxSize  = 100 * 1024 * 1024 // 100MB default cache size
	keepItems       = 10                // Number of items to keep when flushing
)

// BoltStorageInterface defines the methods for BoltStorage
type BoltStorageInterface interface {
	SaveContent(content *types.ClipboardContent) error
	GetLatestContent() (*types.ClipboardContent, error)
	GetContentSince(since time.Time) ([]*types.ClipboardContent, error)
	GetAllContents() ([]*types.ClipboardContent, error)
	GetContentToFlush() ([]*types.ClipboardContent, error)
	DeleteContents(contents []*types.ClipboardContent) error
	Close() error
	GetCacheSize() int64
	FlushCache() error
	GetHistory(options config.HistoryOptions) ([]*types.ClipboardContent, error)
	LogCompleteHistory(options config.HistoryOptions) error
}

// BoltStorage implements persistent storage for clipboard contents using BoltDB
type BoltStorage struct {
	db        *bbolt.DB
	cacheSize int64
	maxSize   int64
	logger    *zap.Logger
	deviceID  string
	keepItems int
}

// StorageConfig holds configuration for BoltStorage initialization
type StorageConfig struct {
	DBPath    string
	MaxSize   int64
	DeviceID  string
	Logger    *zap.Logger
	KeepItems int
}

// NewBoltStorage creates a new BoltStorage instance
func NewBoltStorage(config StorageConfig) (*BoltStorage, error) {
	// Validate and set default values
	maxSize := config.MaxSize
	if maxSize <= 0 {
		maxSize = defaultMaxSize
	}

	keepItemsValue := config.KeepItems
	if keepItemsValue <= 0 {
		keepItemsValue = keepItems
	}

	// Open the database
	db, err := bbolt.Open(config.DBPath, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open bolt database: %w", err)
	}

	// Create bucket if it doesn't exist
	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(clipboardBucket))
		if err != nil {
			return fmt.Errorf("failed to create bucket: %w", err)
		}
		return nil
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create bucket: %w", err)
	}

	// Calculate current cache size
	var cacheSize int64
	err = db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(clipboardBucket))
		return b.ForEach(func(k, v []byte) error {
			atomic.AddInt64(&cacheSize, int64(len(k)+len(v)))
			return nil
		})
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to calculate cache size: %w", err)
	}

	// Create and return the storage implementation
	storage := &BoltStorage{
		db:        db,
		cacheSize: cacheSize,
		maxSize:   maxSize,
		logger:    config.Logger,
		deviceID:  config.DeviceID,
		keepItems: keepItemsValue,
	}

	if config.Logger != nil {
		config.Logger.Debug("BoltStorage initialized", 
			zap.String("db_path", config.DBPath), 
			zap.Int64("max_size", maxSize),
			zap.Int64("current_size", cacheSize))
	}

	return storage, nil
}

// SaveContent saves a clipboard content item to the database
func (s *BoltStorage) SaveContent(content *types.ClipboardContent) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(clipboardBucket))

		encoded, err := json.Marshal(content)
		if err != nil {
			return fmt.Errorf("failed to encode content: %w", err)
		}

		// Check cache size before adding
		newSize := atomic.AddInt64(&s.cacheSize, int64(len(encoded)))
		if newSize > s.maxSize {
			if err := s.flushOldestContent(tx); err != nil {
				s.logger.Error("Failed to flush cache", zap.Error(err))
			}
		}

		return b.Put([]byte(content.Created.Format(time.RFC3339Nano)), encoded)
	})
}

// GetLatestContent retrieves the most recent clipboard content
func (s *BoltStorage) GetLatestContent() (*types.ClipboardContent, error) {
	var content *types.ClipboardContent
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(clipboardBucket))
		c := b.Cursor()
		_, v := c.Last()
		if v == nil {
			return nil
		}
		content = &types.ClipboardContent{}
		return json.Unmarshal(v, content)
	})

	if err != nil {
		return nil, err
	}

	if content != nil && content.Compressed {
		decompressed, err := compression.DecompressContent(content)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress content: %w", err)
		}
		content = decompressed
	}

	return content, nil
}

// GetContentSince retrieves all clipboard content since the specified time
func (s *BoltStorage) GetContentSince(since time.Time) ([]*types.ClipboardContent, error) {
	var contents []*types.ClipboardContent
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(clipboardBucket))
		c := b.Cursor()
		min := []byte(since.Format(time.RFC3339Nano))

		for k, v := c.Seek(min); k != nil; k, v = c.Next() {
			var content types.ClipboardContent
			if err := json.Unmarshal(v, &content); err != nil {
				return fmt.Errorf("failed to decode content: %w", err)
			}

			if content.Compressed {
				decompressed, err := compression.DecompressContent(&content)
				if err != nil {
					return fmt.Errorf("failed to decompress content: %w", err)
				}
				contents = append(contents, decompressed)
			} else {
				contents = append(contents, &content)
			}
		}
		return nil
	})

	return contents, err
}

// GetAllContents retrieves all clipboard content in the database
func (s *BoltStorage) GetAllContents() ([]*types.ClipboardContent, error) {
	return s.GetContentSince(time.Time{}) // Empty time means get everything
}

// GetContentToFlush identifies content that should be flushed from storage
// based on the configured keepItems limit
func (s *BoltStorage) GetContentToFlush() ([]*types.ClipboardContent, error) {
	var itemsToFlush []*types.ClipboardContent

	err := s.db.View(func(tx *bbolt.Tx) error {
		var err error
		itemsToFlush, err = s.collectItemsToFlush(tx)
		return err
	})

	return itemsToFlush, err
}

// collectItemsToFlush finds content that should be flushed from the database
func (s *BoltStorage) collectItemsToFlush(tx *bbolt.Tx) ([]*types.ClipboardContent, error) {
	b := tx.Bucket([]byte(clipboardBucket))
	c := b.Cursor()
	
	// Collect the latest items to keep
	var keepKeys [][]byte
	for k, _ := c.Last(); k != nil && len(keepKeys) < s.keepItems; k, _ = c.Prev() {
		keepKeys = append(keepKeys, k)
	}
	
	// Collect items to be flushed
	var itemsToFlush []*types.ClipboardContent
	for k, v := c.First(); k != nil; k, v = c.Next() {
		shouldKeep := false
		for _, keep := range keepKeys {
			if bytes.Equal(k, keep) {
				shouldKeep = true
				break
			}
		}
		
		if !shouldKeep {
			var content types.ClipboardContent
			if err := json.Unmarshal(v, &content); err != nil {
				s.logger.Error("Failed to unmarshal content", zap.Error(err))
				continue
			}
			itemsToFlush = append(itemsToFlush, &content)
		}
	}
	
	return itemsToFlush, nil
}

// DeleteContents removes specified content items from storage
func (s *BoltStorage) DeleteContents(contents []*types.ClipboardContent) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		return s.deleteItemsFromBucket(tx, contents)
	})
}

// deleteItemsFromBucket removes the specified items from the database
func (s *BoltStorage) deleteItemsFromBucket(tx *bbolt.Tx, itemsToDelete []*types.ClipboardContent) error {
	if len(itemsToDelete) == 0 {
		return nil
	}

	b := tx.Bucket([]byte(clipboardBucket))
	
	// Delete the items
	var totalFreed int64
	for _, content := range itemsToDelete {
		key := []byte(content.Created.Format(time.RFC3339Nano))
		if err := b.Delete(key); err != nil {
			return err
		}
		totalFreed += int64(len(content.Data))
	}
	
	// Update cache size
	atomic.AddInt64(&s.cacheSize, -totalFreed)
	s.logger.Info("Cache items deleted",
		zap.Int64("freed_bytes", totalFreed),
		zap.Int("deleted_items", len(itemsToDelete)))
	
	return nil
}

// flushOldestContent flushes the oldest content from the cache
func (s *BoltStorage) flushOldestContent(tx *bbolt.Tx) error {
	itemsToFlush, err := s.collectItemsToFlush(tx)
	if err != nil {
		return err
	}
	
	return s.deleteItemsFromBucket(tx, itemsToFlush)
}

// FlushCache flushes the oldest content from the cache to stay under size limits
func (s *BoltStorage) FlushCache() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		return s.flushOldestContent(tx)
	})
}

// GetCacheSize returns the current size of the cache in bytes
func (s *BoltStorage) GetCacheSize() int64 {
	return atomic.LoadInt64(&s.cacheSize)
}

// Close closes the database connection
func (s *BoltStorage) Close() error {
	// Attempt to flush before closing
	if err := s.FlushCache(); err != nil {
		s.logger.Error("Failed to flush cache on close", zap.Error(err))
	}
	return s.db.Close()
}

// GetHistory retrieves clipboard history based on the provided options
func (s *BoltStorage) GetHistory(options config.HistoryOptions) ([]*types.ClipboardContent, error) {
	var contents []*types.ClipboardContent
	
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(clipboardBucket))
		c := b.Cursor()
		
		// Determine starting position and iteration direction based on options
		var k, v []byte
		var iterateNext func() ([]byte, []byte)
		
		if options.Reverse {
			// Start from the newest entry if going in reverse
			if !options.Before.IsZero() {
				// Start from the entry just before the 'Before' time
				seekKey := []byte(options.Before.Format(time.RFC3339Nano))
				k, v = c.Seek(seekKey)
				// If we found an exact match or a later key, step back one
				if k != nil && bytes.Compare(k, seekKey) >= 0 {
					k, v = c.Prev()
				}
			} else {
				// No 'Before' specified, start from the very last entry
				k, v = c.Last()
			}
			iterateNext = c.Prev
		} else {
			// Start from the oldest entry if going in forward direction
			if !options.Since.IsZero() {
				// Start from entries at or after the 'Since' time
				seekKey := []byte(options.Since.Format(time.RFC3339Nano))
				k, v = c.Seek(seekKey)
			} else {
				// No 'Since' specified, start from the very first entry
				k, v = c.First()
			}
			iterateNext = c.Next
		}
		
		// Iterate through entries
		count := int64(0)
		for ; k != nil; k, v = iterateNext() {
			// Check time boundaries
			timestamp, err := time.Parse(time.RFC3339Nano, string(k))
			if err != nil {
				s.logger.Error("Failed to parse timestamp", 
					zap.String("key", string(k)), 
					zap.Error(err))
				continue
			}
			
			if !options.Since.IsZero() && timestamp.Before(options.Since) {
				continue
			}
			
			if !options.Before.IsZero() && timestamp.After(options.Before) {
				continue
			}
			
			// Unmarshal the content
			var content types.ClipboardContent
			if err := json.Unmarshal(v, &content); err != nil {
				s.logger.Error("Failed to unmarshal content", 
					zap.String("key", string(k)), 
					zap.Error(err))
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
			
			// Handle decompression if needed
			if content.Compressed {
				decompressed, err := compression.DecompressContent(&content)
				if err != nil {
					s.logger.Error("Failed to decompress content", 
						zap.String("key", string(k)), 
						zap.Error(err))
					continue
				}
				contents = append(contents, decompressed)
			} else {
				contents = append(contents, &content)
			}
			
			// Check if we've reached the limit
			count++
			if options.Limit > 0 && count >= options.Limit {
				break
			}
		}
		
		return nil
	})
	
	if err != nil {
		return nil, fmt.Errorf("failed to get history: %w", err)
	}
	
	return contents, nil
}

// LogCompleteHistory dumps the clipboard history to the logger based on provided options
func (s *BoltStorage) LogCompleteHistory(options config.HistoryOptions) error {
	// If no specific options provided, use defaults for a complete history dump
	if (options == config.HistoryOptions{}) {
		options = config.HistoryOptions{
			Reverse: false, // Oldest first for chronological display
		}
	}
	
	s.logger.Info("=== DUMPING CLIPBOARD HISTORY ===",
		zap.Any("limit", optionOrDefault(options.Limit, "no limit")),
		zap.Bool("reverse", options.Reverse),
		zap.String("content_type", optionOrDefault(options.ContentType, "all types")))
	
	// Get filtered history
	contents, err := s.GetHistory(options)
	if err != nil {
		return fmt.Errorf("failed to get history: %w", err)
	}
	
	// Log each history item
	for i, content := range contents {
		// Format preview based on content type
		var preview string
		switch content.Type {
		case types.TypeImage:
			preview = fmt.Sprintf("[Image: %d bytes]", len(content.Data))
		case types.TypeFile:
			preview = fmt.Sprintf("[File: %s]", string(content.Data))
		case types.TypeURL:
			preview = fmt.Sprintf("[URL: %s]", string(content.Data))
		case types.TypeFilePath:
			preview = fmt.Sprintf("[Path: %s]", string(content.Data))
		default:
			// For text, show preview
			if len(content.Data) > 100 {
				preview = fmt.Sprintf("%s... (%d more bytes)", 
					string(content.Data[:100]), 
					len(content.Data)-100)
			} else {
				preview = string(content.Data)
			}
		}
		
		s.logger.Info(fmt.Sprintf("History item %d:", i+1),
			zap.String("timestamp", content.Created.Format(time.RFC3339)),
			zap.String("type", string(content.Type)),
			zap.Int("size", len(content.Data)),
			zap.Bool("compressed", content.Compressed),
			zap.String("content", preview))
	}
	
	s.logger.Info("=== END OF CLIPBOARD HISTORY ===", 
		zap.Int("total_items", len(contents)),
		zap.Int64("total_size_bytes", s.GetCacheSize()))
		
	return nil
}

// Helper function to display option values or defaults
func optionOrDefault(value interface{}, defaultText string) string {
	switch v := value.(type) {
	case int:
		if v == 0 {
			return defaultText
		}
		return fmt.Sprintf("%d", v)
	case string:
		if v == "" {
			return defaultText
		}
		return v
	default:
		return fmt.Sprintf("%v", value)
	}
}
