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
	c.mu.Lock()
	defer c.mu.Unlock()

	if content == nil || len(content.Data) == 0 {
		return fmt.Errorf("no content to write to clipboard")
	}

	var err error

	switch content.Type {
	case types.TypeText, types.TypeString:
		err = writeText(string(content.Data))
		c.logger.Info("Wrote text to clipboard", zap.Int("size", len(content.Data)))
	case types.TypeHTML:
		err = writeHTML(string(content.Data))
		c.logger.Info("Wrote HTML to clipboard", zap.Int("size", len(content.Data)))
	case types.TypeRTF:
		err = writeRTF(content.Data)
		c.logger.Info("Wrote RTF to clipboard", zap.Int("size", len(content.Data)))
	case types.TypeImage:
		err = writeImage(content.Data)
		c.logger.Info("Wrote image to clipboard", zap.Int("size", len(content.Data)))
	case types.TypeFile:
		// Expecting JSON-encoded file list
		var files []string
		if uerr := json.Unmarshal(content.Data, &files); uerr != nil {
			err = fmt.Errorf("invalid file list data: %w", uerr)
		} else {
			err = writeFileList(files)
			c.logger.Info("Wrote file list to clipboard", zap.Int("count", len(files)))
		}
	case types.TypeURL:
		err = writeURL(string(content.Data))
		c.logger.Info("Wrote URL to clipboard", zap.String("url", string(content.Data)))
	default:
		err = fmt.Errorf("unsupported content type: %s", content.Type)
	}

	// Always clear the cache after writing
	c.clearCache()

	if err != nil {
		c.logger.Warn("Failed to write to clipboard", zap.Error(err), zap.String("type", string(content.Type)))
		return err
	}
	
	// update lastChangeCount to match system 
	c.lastChangeCount = getChangeCount()
	// update cache with just-written content
	c.updateCache(content, hashContent(content.Data), []string{string(content.Type)})

	return nil
}

func (c *DarwinClipboard) MonitorChanges(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
	go func() {
		interval := c.baseInterval
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-stopCh:
				c.logger.Info("Clipboard monitoring stopped")
				return
			case <-ticker.C:
				currentChangeCount := getChangeCount()
				if currentChangeCount != c.lastChangeCount {
					c.logger.Debug("Clipboard change detected", zap.Uint64("old", c.lastChangeCount), zap.Uint64("new", currentChangeCount))
					content, err := c.Read()
					if err == nil && content != nil {
						select {
						case contentCh <- content:
							c.logger.Info("New clipboard content sent to channel")
						case <-stopCh:
							c.logger.Info("Clipboard monitoring stopped during send")
							return
						}
					} else if err != nil {
						c.logger.Warn("Failed to read clipboard after change", zap.Error(err))
					}
					c.lastChangeCount = currentChangeCount
					interval = c.baseInterval
					ticker.Reset(interval)
				} else {
					// No change detected, increase interval up to maxInterval
					if interval < c.maxInterval {
						interval = time.Duration(float64(interval) * 1.5)
						if interval > c.maxInterval {
							interval = c.maxInterval
						}
						ticker.Reset(interval)
						c.logger.Debug("Increased polling interval", zap.Duration("interval", interval))
					}
				}
			}
		}
	}()
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

func (c *DarwinClipboard) Close() error {
	var closeErr error

	// 1. Stop monitoring goroutine if applicable
	if c.monitorCancel != nil {
		c.monitorCancel()
		c.logger.Info("Clipboard monitoring cancelled")
	}

	// 2. Close storage backend if present
	if c.storage != nil {
		if err := c.storage.Close(); err != nil {
			c.logger.Warn("Failed to close clipboard storage", zap.Error(err))
			closeErr = err
		} else {
			c.logger.Info("Clipboard storage closed")
		}
	}

	// 3. Clear the in-memory cache
	c.clearCache()
	c.logger.Info("Clipboard cache cleared")

	// 4. Log shutdown
	c.logger.Info("DarwinClipboard closed")

	return closeErr
}