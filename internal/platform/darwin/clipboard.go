//go:build darwin
// +build darwin

package platform

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"go.uber.org/zap"
)

// DarwinClipboard provides a macOS clipboard abstraction
type DarwinClipboard struct {
	mu             sync.Mutex
	lastContent    string
	isRunning      bool
	logger         *zap.Logger
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewClipboard creates a new DarwinClipboard instance
func NewClipboard(logger *zap.Logger) Clipboard {
	if logger == nil {
		logger = zap.NewNop()
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &DarwinClipboard{
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}
}

// NewDaemonizer creates a new DarwinDaemonizer instance
func NewDaemonizer() Daemonizer {
	return &DarwinDaemonizer{}
}

// Read returns the current clipboard content
func (c *DarwinClipboard) Read() (*types.ClipboardContent, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// TODO: Implement using NSPasteboard
	return nil, fmt.Errorf("not implemented")
}

// Write sets the clipboard content
func (c *DarwinClipboard) Write(content *types.ClipboardContent) error {
	if content == nil || len(content.Data) == 0 {
		return fmt.Errorf("no content to write")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// TODO: Implement using NSPasteboard
	return fmt.Errorf("not implemented")
}

// MonitorChanges starts monitoring for clipboard changes
func (c *DarwinClipboard) MonitorChanges(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
	// TODO: Implement using NSPasteboard change count monitoring
}

// Close releases resources
func (c *DarwinClipboard) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isRunning {
		c.cancel()
		c.isRunning = false
	}
} 