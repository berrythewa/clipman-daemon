package storage

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/berrythewa/clipman-daemon/pkg/compression"
	"github.com/berrythewa/clipman-daemon/pkg/utils"

	"go.etcd.io/bbolt"
	"go.uber.org/zap"
)

const (
	clipboardBucket = "clipboard"
	defaultMaxSize  = 100 * 1024 * 1024 // 100MB default cache size
	keepItems       = 10                // Number of items to keep when flushing
	maxOccurrences  = 1000             // Maximum number of occurrences to store per item
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
	GetOccurrenceStats(hash string) (*types.OccurrenceStats, error)
	GetFrequentContent(limit int) ([]*types.ClipboardContent, error)
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
				
				// Limit number of stored occurrences
				if len(existing.Occurrences) > maxOccurrences {
					existing.Occurrences = existing.Occurrences[:maxOccurrences]
				}
				
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
		content.Created = now
		content.Occurrences = []time.Time{now}
		
		s.logger.Debug("New content added",
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
			
			// BACKWARD COMPATIBILITY: Generate hash if missing
			if content.Hash == "" && len(content.Data) > 0 {
				if len(content.Data) > 1024*1024 { // 1MB threshold
					content.Hash = utils.HashContentBis(content.Data)
				} else {
					content.Hash = utils.HashContent(content.Data)
				}
				needsHashUpdate = true
				s.logger.Debug("Generated missing hash for latest content", 
					zap.String("hash", content.Hash),
					zap.String("type", string(content.Type)))
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
			latestContent = s.decodeContentIfNeeded(latestContent)
		}
	}

	return latestContent, nil
}

// GetContentSince retrieves all clipboard content occurrences since the specified time
func (s *BoltStorage) GetContentSince(since time.Time) ([]*types.ClipboardContent, error) {
	var occurrences []struct {
		Content *types.ClipboardContent
		Time    time.Time
	}

	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(clipboardBucket))
		return b.ForEach(func(k, v []byte) error {
			var content types.ClipboardContent
			if err := json.Unmarshal(v, &content); err != nil {
				s.logger.Warn("Failed to unmarshal content", zap.Error(err), zap.Binary("hash", k))
				return nil // skip invalid entries
			}

			// Check each occurrence
			for _, occ := range content.Occurrences {
				if occ.After(since) || occ.Equal(since) {
					copyContent := content
					copyContent.Created = occ
					occurrences = append(occurrences, struct {
						Content *types.ClipboardContent
						Time    time.Time
					}{&copyContent, occ})
				}
			}
			return nil
		})
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get content since %v: %w", since, err)
	}

	// Sort by occurrence time ascending
	sort.Slice(occurrences, func(i, j int) bool {
		return occurrences[i].Time.Before(occurrences[j].Time)
	})

	// Flatten to []*ClipboardContent and handle compression
	result := make([]*types.ClipboardContent, 0, len(occurrences))
	for _, occ := range occurrences {
		// ALWAYS attempt decompression for backward compatibility
		decompressed, err := compression.DecompressContent(occ.Content)
		if err == nil {
			// Decompression succeeded
			s.logger.Debug("Successfully decompressed content since", 
				zap.String("hash", occ.Content.Hash),
				zap.Bool("compressed_flag", occ.Content.Compressed))
			result = append(result, decompressed)
		} else {
			// Decompression failed - try base64 decoding as fallback  
			s.logger.Debug("Decompression failed for content since, trying base64 decode",
				zap.String("hash", occ.Content.Hash),
				zap.Error(err))
			decoded := s.decodeContentIfNeeded(occ.Content)
			result = append(result, decoded)
		}
	}

	return result, nil
}

// GetAllContents retrieves all clipboard content occurrences in the database
func (s *BoltStorage) GetAllContents() ([]*types.ClipboardContent, error) {
	return s.GetContentSince(time.Time{}) // All occurrences
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
			processedContent = s.decodeContentIfNeeded(content)
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

// LogCompleteHistory dumps the clipboard history to the logger based on provided options
func (s *BoltStorage) LogCompleteHistory(options config.HistoryOptions) error {
	// If no specific options provided, use defaults for a complete history dump
	if (options == config.HistoryOptions{}) {
		options = config.HistoryOptions{
			Reverse: false, // Oldest first for chronological display
		}
	}
	
	// Print a header with filter information
	limitInfo := optionOrDefault(options.Limit, "no limit")
	typeInfo := optionOrDefault(options.ContentType, "all types")
	reverseInfo := ""
	if options.Reverse {
		reverseInfo = " (newest first)"
	} else {
		reverseInfo = " (oldest first)"
	}
	
	fmt.Printf("\n=== CLIPBOARD HISTORY %s===\n", reverseInfo)
	fmt.Printf("Filters: limit=%s, type=%s\n", limitInfo, typeInfo)
	
	if options.MinSize > 0 {
		fmt.Printf("         min_size=%d bytes\n", options.MinSize)
	}
	
	if !options.Since.IsZero() {
		fmt.Printf("         since=%s\n", options.Since.Format(time.RFC3339))
	}
	
	if !options.Before.IsZero() {
		fmt.Printf("         before=%s\n", options.Before.Format(time.RFC3339))
	}
	
	fmt.Println("")
	
	// Get filtered history
	contents, err := s.GetHistory(options)
	if err != nil {
		return fmt.Errorf("failed to get history: %w", err)
	}
	
	if len(contents) == 0 {
		fmt.Println("No clipboard history matching the specified filters.")
		return nil
	}
	
	// Log each history item
	for i, content := range contents {
		// Format timestamp for display
		timestampStr := content.Created.Format("2006-01-02 15:04:05")
		
		// Create a header for each item
		var itemHeader string
		if options.Reverse {
			// For reverse order, show age relative to now
			age := formatTimeAgo(time.Since(content.Created))
			
			if options.Limit == 1 || (i == 0 && len(contents) <= 3) {
				// For single-item limit or small result set, highlight the most recent item
				itemHeader = fmt.Sprintf("MOST RECENT ITEM (%s ago):", age)
			} else {
				itemHeader = fmt.Sprintf("ITEM %d (%s ago):", i+1, age)
			}
		} else {
			// For chronological order
			if i == len(contents)-1 && len(contents) > 1 {
				// Highlight the last (most recent) item in chronological display
				age := formatTimeAgo(time.Since(content.Created))
				itemHeader = fmt.Sprintf("MOST RECENT ITEM %d of %d (%s ago):", i+1, len(contents), age)
			} else {
				itemHeader = fmt.Sprintf("ITEM %d of %d:", i+1, len(contents))
			}
		}
		
		fmt.Printf("\n%s\n", itemHeader)
		fmt.Printf("  Timestamp: %s\n", timestampStr)
		fmt.Printf("  Type: %s\n", content.Type)
		fmt.Printf("  Size: %d bytes\n", len(content.Data))
		
		// Format preview based on content type
		fmt.Println("  Content:")
		
		switch content.Type {
		case types.TypeImage:
			fmt.Println("    [Binary image data]")
		
		case types.TypeFile:
			// For file lists, try to parse JSON and display in a nicer format
			var files []string
			if err := json.Unmarshal(content.Data, &files); err == nil && len(files) > 0 {
				if len(files) == 1 {
					fmt.Printf("    [File: %s]\n", files[0])
				} else {
					// Show first few files for multiple files
					fmt.Printf("    [Files: %d total]\n", len(files))
					maxFilesToShow := 3
					if len(files) < maxFilesToShow {
						maxFilesToShow = len(files)
					}
					
					for i := 0; i < maxFilesToShow; i++ {
						fmt.Printf("    - %s\n", files[i])
					}
					
					if len(files) > maxFilesToShow {
						fmt.Printf("    - ... and %d more files\n", len(files)-maxFilesToShow)
					}
				}
			} else {
				// If we couldn't parse as JSON, show formatted text
				fmt.Println(formatTextPreviewWithIndent(string(content.Data), 120, 4))
			}
		
		case types.TypeURL:
			url := strings.TrimSpace(string(content.Data))
			fmt.Printf("    %s\n", url)
		
		case types.TypeFilePath:
			path := strings.TrimSpace(string(content.Data))
			fmt.Printf("    %s\n", path)
			
			// Check if file exists and add info
			if _, err := os.Stat(path); err == nil {
				fmt.Printf("    (File exists)\n")
			} else {
				fmt.Printf("    (File not found: %v)\n", err)
			}
		
		default:
			// For text content, show a nicely formatted preview
			fmt.Println(formatTextPreviewWithIndent(string(content.Data), 120, 4))
		}
	}
	
	fmt.Printf("\n=== END OF CLIPBOARD HISTORY (%d items) ===\n\n", len(contents))
	
	return nil
}

// formatTextPreview formats a text string for display, handling newlines and length limits
func formatTextPreview(text string, maxLength int, indentSpaces int) string {
	// Replace tabs with spaces
	text = strings.Replace(text, "\t", "  ", -1)
	
	// Create indent string
	indent := strings.Repeat(" ", indentSpaces)
	
	// Handle multi-line content
	lines := strings.Split(text, "\n")
	var builder strings.Builder
	
	// Determine how many lines to show
	maxLines := 15 // Maximum number of lines to display
	showLines := len(lines)
	if showLines > maxLines {
		showLines = maxLines
	}
	
	// Add each line with indentation
	for i := 0; i < showLines; i++ {
		line := strings.TrimRight(lines[i], " \t\r\n")
		
		if len(line) > maxLength {
			line = line[:maxLength] + "..."
		}
		
		builder.WriteString(indent)
		builder.WriteString(line)
		builder.WriteString("\n")
	}
	
	// If we truncated lines, show a count
	if len(lines) > maxLines {
		builder.WriteString(indent)
		builder.WriteString(fmt.Sprintf("... (%d more lines not shown)\n", len(lines)-maxLines))
	}
	
	return builder.String()
}

// formatTextPreviewWithIndent formats text with proper indentation
func formatTextPreviewWithIndent(text string, maxLength int, indentSpaces int) string {
	// Replace tabs with spaces
	text = strings.Replace(text, "\t", "  ", -1)
	
	// Create indent string
	indent := strings.Repeat(" ", indentSpaces)
	
	// Handle multi-line content
	lines := strings.Split(text, "\n")
	var builder strings.Builder
	
	// Determine how many lines to show
	maxLines := 15 // Maximum number of lines to display
	showLines := len(lines)
	if showLines > maxLines {
		showLines = maxLines
	}
	
	// Add each line with indentation
	for i := 0; i < showLines; i++ {
		line := strings.TrimRight(lines[i], " \t\r\n")
		
		if len(line) > maxLength {
			line = line[:maxLength] + "..."
		}
		
		builder.WriteString(indent)
		builder.WriteString(line)
		builder.WriteString("\n")
	}
	
	// If we truncated lines, show a count
	if len(lines) > maxLines {
		builder.WriteString(indent)
		builder.WriteString(fmt.Sprintf("... (%d more lines not shown)\n", len(lines)-maxLines))
	}
	
	return builder.String()
}

// formatTimeAgo formats a duration as a human-readable "time ago" string
func formatTimeAgo(duration time.Duration) string {
	seconds := int(duration.Seconds())
	
	if seconds < 60 {
		return fmt.Sprintf("%d seconds", seconds)
	}
	
	minutes := seconds / 60
	if minutes < 60 {
		return fmt.Sprintf("%d minutes", minutes)
	}
	
	hours := minutes / 60
	if hours < 24 {
		return fmt.Sprintf("%d hours", hours)
	}
	
	days := hours / 24
	if days < 30 {
		return fmt.Sprintf("%d days", days)
	}
	
	months := days / 30
	if months < 12 {
		return fmt.Sprintf("%d months", months)
	}
	
	years := months / 12
	return fmt.Sprintf("%d years", years)
}

// Helper function to display option values or defaults
func optionOrDefault(value interface{}, defaultText string) string {
	switch v := value.(type) {
	case int64:
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

// GetOccurrenceStats returns statistics about content occurrences
func (s *BoltStorage) GetOccurrenceStats(hash string) (*types.OccurrenceStats, error) {
	var stats *types.OccurrenceStats
	
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(clipboardBucket))
		v := b.Get([]byte(hash))
		if v == nil {
			return fmt.Errorf("content not found")
		}
		
		var content types.ClipboardContent
		if err := json.Unmarshal(v, &content); err != nil {
			return fmt.Errorf("failed to unmarshal content: %w", err)
		}
		
		// Calculate statistics
		stats = &types.OccurrenceStats{
			Hash:            hash,
			TotalOccurrences: len(content.Occurrences),
			FirstSeen:       content.Occurrences[len(content.Occurrences)-1],
			LastSeen:        content.Occurrences[0],
			ContentType:     content.Type,
		}
		
		// Calculate frequency if we have multiple occurrences
		if len(content.Occurrences) > 1 {
			duration := content.Occurrences[0].Sub(content.Occurrences[len(content.Occurrences)-1])
			stats.AverageFrequency = duration / time.Duration(len(content.Occurrences)-1)
		}
		
		return nil
	})
	
	if err != nil {
		return nil, fmt.Errorf("failed to get occurrence stats: %w", err)
	}
	
	return stats, nil
}

// GetFrequentContent returns the most frequently occurring content
func (s *BoltStorage) GetFrequentContent(limit int) ([]*types.ClipboardContent, error) {
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
			result[i] = s.decodeContentIfNeeded(item.content)
		}
	}
	
	return result, nil
}

// decodeContentIfNeeded checks if content.Data is base64 encoded and decodes it if it is
func (s *BoltStorage) decodeContentIfNeeded(content *types.ClipboardContent) *types.ClipboardContent {
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
