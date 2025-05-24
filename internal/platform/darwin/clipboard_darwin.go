// internal/platform/clipboard_darwin.go
// +build darwin

package platform

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/berrythewa/clipman-daemon/pkg/utils"
	"go.uber.org/zap"
)

// DarwinClipboard is the macOS-specific clipboard implementation
type DarwinClipboard struct {
	lastChangeCount uint64
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



// Clipboard interface methods (to be implemented)
func (c *DarwinClipboard) Read() (*types.ClipboardContent, error) {
	// 1. Check for change
	currentChangeCount := getChangeCount()
	if currentChangeCount == c.lastChangeCount {
		c.logger.Debug("Clipboard content unchanged (change count)")
		return nil, fmt.Errorf("content unchanged")
	}

	// 3. Try to read each supported type (priority order)
	// HTML
	if html, err := readHTML(); err == nil && html != "" {
		content := utils.newClipboardContent(types.TypeHTML, []byte(html))
		c.logger.Info("Read HTML from clipboard", zap.Int("size", len(content.Data)))
		return content, nil
	}
	// RTF
	if rtf, err := readRTF(); err == nil && len(rtf) > 0 {
		content := utils.newClipboardContent(types.TypeRTF, rtf)
		c.logger.Info("Read RTF from clipboard", zap.Int("size", len(content.Data)))
		return content, nil
	}
	// Image
	if img, err := readImage(); err == nil && len(img) > 0 {
		content := utils.newClipboardContent(types.TypeImage, img)
		c.logger.Info("Read image from clipboard", zap.Int("size", len(content.Data)))
		return content, nil
	}
	// File List
	if files, err := readFileList(); err == nil && len(files) > 0 {
		// Serialize as JSON
		fileJSON, ferr := json.Marshal(files)
		if ferr == nil {
			content := utils.newClipboardContent(types.TypeFile, fileJSON)
			c.logger.Info("Read file list from clipboard", zap.Int("count", len(files)))
			return content, nil
		}
	}
	// URL
	if url, err := readURL(); err == nil && url != "" {
		content := utils.newClipboardContent(types.TypeURL, []byte(url))
		c.logger.Info("Read URL from clipboard", zap.String("url", url))
		return content, nil
	}
	// Text (fallback)
	if text, err := readText(); err == nil && text != "" {
		// Optionally: Detect if it's a file path, URL, etc.
		content := utils.newClipboardContent(types.TypeText, []byte(text))
		c.logger.Info("Read text from clipboard", zap.Int("size", len(content.Data)))
		return content, nil
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

	if err != nil {
		c.logger.Warn("Failed to write to clipboard", zap.Error(err), zap.String("type", string(content.Type)))
		return err
	}
	
	// update lastChangeCount to match system 
	c.lastChangeCount = getChangeCount()

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

func (c *DarwinClipboard) Close() error {
	var closeErr error

	// 1. Stop monitoring goroutine if applicable
	if c.monitorCancel != nil {
		c.monitorCancel()
		c.logger.Info("Clipboard monitoring cancelled")
	}

	// 4. Log shutdown
	c.logger.Info("DarwinClipboard closed")

	return closeErr
}