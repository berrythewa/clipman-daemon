//go:build windows
// +build windows

package platform

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"go.uber.org/zap"
	"golang.org/x/sys/windows"
)

// WindowsClipboard provides a Windows clipboard abstraction
type WindowsClipboard struct {
	mu             sync.Mutex
	lastContent    string
	isRunning      bool
	logger         *zap.Logger
	ctx            context.Context
	cancel         context.CancelFunc
	hwnd           windows.Handle
}

// NewClipboard creates a new WindowsClipboard instance
func NewClipboard(logger *zap.Logger) Clipboard {
	if logger == nil {
		logger = zap.NewNop()
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &WindowsClipboard{
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}
}

// NewDaemonizer creates a new WindowsDaemonizer instance
func NewDaemonizer() Daemonizer {
	return &WindowsDaemonizer{}
}

// Read returns the current clipboard content
func (c *WindowsClipboard) Read() (*types.ClipboardContent, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// TODO: Implement using Windows Clipboard API
	return nil, fmt.Errorf("not implemented")
}

// Write sets the clipboard content
func (c *WindowsClipboard) Write(content *types.ClipboardContent) error {
	if content == nil || len(content.Data) == 0 {
		return fmt.Errorf("no content to write")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	// TODO: Implement using Windows Clipboard API
	return fmt.Errorf("not implemented")
}

// MonitorChanges starts monitoring for clipboard changes
func (c *WindowsClipboard) MonitorChanges(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
	// TODO: Implement using Windows Clipboard API
}

// Close releases resources
func (c *WindowsClipboard) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isRunning {
		c.cancel()
		c.isRunning = false
	}

	if c.hwnd != 0 {
		windows.DestroyWindow(c.hwnd)
		c.hwnd = 0
	}
} 