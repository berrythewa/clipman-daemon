// internal/platform/clipboard_darwin.go
// +build darwin

package platform

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"go.uber.org/zap"
)

// ContentCache struct (reuse from Linux if possible)
type ContentCache struct {
	Content         *types.ClipboardContent
	Hash            string
	FormatsHash     string
	Formats         []string
	LastAccessTime  time.Time
	ExpirationTime  time.Duration
}

// DarwinClipboard is the macOS-specific clipboard implementation
type DarwinClipboard struct {
	lastChangeCount uint64
	cache           *ContentCache
	cacheMutex      sync.RWMutex
	cacheEnabled    bool
	cacheExpiry     time.Duration
	logger          *zap.Logger
	baseInterval    time.Duration
	maxInterval     time.Duration
	currentInterval time.Duration
	inactiveStreak  int
	stealthMode     bool
	mu              sync.Mutex
}

// NewClipboard creates a new platform-specific clipboard implementation
func NewClipboard() *DarwinClipboard {
	return &DarwinClipboard{
		lastChangeCount: getChangeCount(),
		cacheEnabled:    true,
		cacheExpiry:     2 * time.Second,
		logger:          zap.NewNop(), // Replace with zap.L() or a configured logger as needed
		baseInterval:    500 * time.Millisecond,
		maxInterval:     5 * time.Second,
		currentInterval: 500 * time.Millisecond,
	}
}

// SetLogger allows setting a custom zap logger
func (c *DarwinClipboard) SetLogger(logger *zap.Logger) {
	if logger != nil {
		c.logger = logger
	}
}

// SetStealthMode sets the stealth mode option
func (c *DarwinClipboard) SetStealthMode(enabled bool) {
	c.stealthMode = enabled
}

// SetPollingIntervals sets the polling intervals in milliseconds
func (c *DarwinClipboard) SetPollingIntervals(baseMs, maxMs int64) {
	if baseMs < 100 {
		baseMs = 100
	}
	if maxMs < baseMs {
		maxMs = baseMs * 3
	}
	c.baseInterval = time.Duration(baseMs) * time.Millisecond
	c.maxInterval = time.Duration(maxMs) * time.Millisecond
}

// EnableCache enables or disables the content cache
func (c *DarwinClipboard) EnableCache(enabled bool) {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()
	c.cacheEnabled = enabled
}

// SetCacheExpiry sets the cache expiration time in milliseconds
func (c *DarwinClipboard) SetCacheExpiry(expiryMs int64) {
	if expiryMs < 100 {
		expiryMs = 100
	}
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()
	c.cacheExpiry = time.Duration(expiryMs) * time.Millisecond
}

// clearCache clears the content cache
func (c *DarwinClipboard) clearCache() {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()
	c.cache = nil
}

// isCacheValid checks if the cache is valid
func (c *DarwinClipboard) isCacheValid() bool {
	c.cacheMutex.RLock()
	defer c.cacheMutex.RUnlock()
	if !c.cacheEnabled || c.cache == nil {
		return false
	}
	return time.Since(c.cache.LastAccessTime) < c.cacheExpiry
}

// getCachedContent gets the cached content if valid
func (c *DarwinClipboard) getCachedContent() *types.ClipboardContent {
	if !c.isCacheValid() {
		return nil
	}
	c.cacheMutex.RLock()
	defer c.cacheMutex.RUnlock()
	orig := c.cache.Content
	dataCopy := make([]byte, len(orig.Data))
	copy(dataCopy, orig.Data)
	return &types.ClipboardContent{
		Type:       orig.Type,
		Data:       dataCopy,
		Created:    orig.Created,
		Compressed: orig.Compressed,
	}
}

// updateCache updates the cache with new content
func (c *DarwinClipboard) updateCache(content *types.ClipboardContent, contentHash string, formats []string) {
	if !c.cacheEnabled {
		return
	}
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()
	formatsHash := ""
	if len(formats) > 0 {
		formatsHash = joinFormats(formats)
	}
	c.cache = &ContentCache{
		Content:        content,
		Hash:           contentHash,
		FormatsHash:    formatsHash,
		Formats:        formats,
		LastAccessTime: time.Now(),
		ExpirationTime: c.cacheExpiry,
	}
}

// joinFormats is a helper to join format strings
func joinFormats(formats []string) string {
	result := ""
	for i, f := range formats {
		if i > 0 {
			result += ","
		}
		result += f
	}
	return result
}

// Clipboard interface methods (to be implemented)
func (c *DarwinClipboard) Read() (*types.ClipboardContent, error) {
	// 1. Check cache
	if cached := c.getCachedContent(); cached != nil {
		c.logger.Debug("Returning cached clipboard content")
		return cached, nil
	}

	// 2. Check for change
	currentChangeCount := getChangeCount()
	if currentChangeCount == c.lastChangeCount {
		c.logger.Debug("Clipboard content unchanged (change count)")
		return nil, fmt.Errorf("content unchanged")
	}

	// 3. Try to read each supported type (priority order)
	// HTML
	if html, err := readHTML(); err == nil && html != "" {
		content := &types.ClipboardContent{Type: types.TypeHTML, Data: []byte(html), Created: time.Now()}
		c.updateCache(content, hashContent(content.Data), []string{"html"})
		c.lastChangeCount = currentChangeCount
		c.logger.Info("Read HTML from clipboard", zap.Int("size", len(content.Data)))
		return c.getCachedContent(), nil
	}
	// RTF
	if rtf, err := readRTF(); err == nil && len(rtf) > 0 {
		content := &types.ClipboardContent{Type: types.TypeRTF, Data: rtf, Created: time.Now()}
		c.updateCache(content, hashContent(content.Data), []string{"rtf"})
		c.lastChangeCount = currentChangeCount
		c.logger.Info("Read RTF from clipboard", zap.Int("size", len(content.Data)))
		return c.getCachedContent(), nil
	}
	// Image
	if img, err := readImage(); err == nil && len(img) > 0 {
		content := &types.ClipboardContent{Type: types.TypeImage, Data: img, Created: time.Now()}
		c.updateCache(content, hashContent(content.Data), []string{"image"})
		c.lastChangeCount = currentChangeCount
		c.logger.Info("Read image from clipboard", zap.Int("size", len(content.Data)))
		return c.getCachedContent(), nil
	}
	// File List
	if files, err := readFileList(); err == nil && len(files) > 0 {
		// Serialize as JSON
		fileJSON, ferr := json.Marshal(files)
		if ferr == nil {
			content := &types.ClipboardContent{Type: types.TypeFile, Data: fileJSON, Created: time.Now()}
			c.updateCache(content, hashContent(content.Data), []string{"file"})
			c.lastChangeCount = currentChangeCount
			c.logger.Info("Read file list from clipboard", zap.Int("count", len(files)))
			return c.getCachedContent(), nil
		}
	}
	// URL
	if url, err := readURL(); err == nil && url != "" {
		content := &types.ClipboardContent{Type: types.TypeURL, Data: []byte(url), Created: time.Now()}
		c.updateCache(content, hashContent(content.Data), []string{"url"})
		c.lastChangeCount = currentChangeCount
		c.logger.Info("Read URL from clipboard", zap.String("url", url))
		return c.getCachedContent(), nil
	}
	// Text (fallback)
	if text, err := readText(); err == nil && text != "" {
		// Optionally: Detect if it's a file path, URL, etc.
		content := &types.ClipboardContent{Type: types.TypeText, Data: []byte(text), Created: time.Now()}
		c.updateCache(content, hashContent(content.Data), []string{"text"})
		c.lastChangeCount = currentChangeCount
		c.logger.Info("Read text from clipboard", zap.Int("size", len(content.Data)))
		return c.getCachedContent(), nil
	}

	c.logger.Warn("No supported content in clipboard")
	return nil, fmt.Errorf("no supported content in clipboard")
}

func (c *DarwinClipboard) Write(content *types.ClipboardContent) error {
	// TODO: Implement using helpers and cache clearing
	return nil
}

func (c *DarwinClipboard) MonitorChanges(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
	// TODO: Implement adaptive polling using getChangeCount and Read
}

func (c *DarwinClipboard) FlushCache() {
	c.cacheMutex.Lock()
	defer c.cacheMutex.Unlock()

	if c.cache != nil && c.cache.Content != nil && c.storage != nil {
		latest, err := c.storage.GetLatestContent()
		shouldSave := false

		if err != nil || latest == nil {
			// Nothing in storage, must save
			shouldSave = true
		} else if !latest.Equal(c.cache.Content) {
			// Content differs, must update
			shouldSave = true
		}

		if shouldSave {
			if err := c.storage.SaveContent(c.cache.Content); err != nil {
				c.logger.Warn("Failed to save clipboard content to storage before flush", zap.Error(err))
			} else {
				c.logger.Info("Clipboard content saved to storage before flush")
			}
		} else {
			c.logger.Debug("Clipboard content already up to date in storage")
		}
	}

	c.cache = nil
	c.logger.Info("Clipboard cache flushed")
}

func (c *DarwinClipboard) Close() {
	// Stop monitoring if you have a cancel function or stop channel
	// if c.monitorCancel != nil {
	//     c.monitorCancel()
	// }

	// Clear the in-memory cache
	c.clearCache()

	c.logger.Info("DarwinClipboard closed, cache cleared")
}