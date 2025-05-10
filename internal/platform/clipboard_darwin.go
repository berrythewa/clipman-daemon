// internal/platform/clipboard_darwin.go
// +build darwin

package platform

import (
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
	//1. Get cached content
	
	//2. If cached content is valid, return it
	//3. If cached content is not valid, read from clipboard
	//4. Update cache with new content
	//5. Return new content

	return nil, nil
}

func (c *DarwinClipboard) Write(content *types.ClipboardContent) error {
	// TODO: Implement using helpers and cache clearing
	return nil
}

func (c *DarwinClipboard) MonitorChanges(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
	// TODO: Implement adaptive polling using getChangeCount and Read
}

func (c *DarwinClipboard) Close() {
	// Nothing to clean up for Darwin clipboard
}