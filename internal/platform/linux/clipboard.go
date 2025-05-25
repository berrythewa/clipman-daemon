//go:build linux
// +build linux

package platform

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"go.uber.org/zap"
	"github.com/atotto/clipboard"
)

// ClipboardChangeHandler is a callback for new clipboard content
type ClipboardChangeHandler func(content *types.ClipboardContent)

// Clipboard defines the interface for clipboard operations
type Clipboard interface {
	Read() (*types.ClipboardContent, error)
	Write(content *types.ClipboardContent) error
	MonitorChanges(handler ClipboardChangeHandler)
	Close()
}

// LinuxClipboard provides a Linux clipboard abstraction
type LinuxClipboard struct {
	mu             sync.Mutex
	lastContent    string
	isRunning      bool
	baseInterval   time.Duration
	maxInterval    time.Duration
	inactiveStreak int
	logger         *zap.Logger
	stealthMode    bool
	ctx            context.Context
	cancel         context.CancelFunc
}

// NewClipboard creates a new LinuxClipboard instance
func NewClipboard(logger *zap.Logger) Clipboard {
	if logger == nil {
		logger = zap.NewNop()
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &LinuxClipboard{
		baseInterval: 500 * time.Millisecond,
		maxInterval:  2 * time.Second,
		logger:      logger,
		ctx:         ctx,
		cancel:      cancel,
	}
}

// SetPollingIntervals sets the polling intervals for clipboard monitoring
func (c *LinuxClipboard) SetPollingIntervals(baseMs, maxMs int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.baseInterval = time.Duration(baseMs) * time.Millisecond
	c.maxInterval = time.Duration(maxMs) * time.Millisecond
	c.logger.Debug("Updated polling intervals",
		zap.Duration("base", c.baseInterval),
		zap.Duration("max", c.maxInterval))
}

// Read returns the current clipboard content
func (c *LinuxClipboard) Read() (*types.ClipboardContent, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	text, err := clipboard.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read clipboard: %w", err)
	}

	// Detect content type and create content
	content, err := detectContent(text)
	if err != nil {
		return nil, fmt.Errorf("failed to detect content: %w", err)
	}

	c.logger.Debug("Read clipboard content",
		zap.String("type", string(content.Type)),
		zap.Int("size", len(content.Data)))

	return content, nil
}

// Write sets the clipboard content
func (c *LinuxClipboard) Write(content *types.ClipboardContent) error {
	if content == nil || len(content.Data) == 0 {
		return fmt.Errorf("no content to write")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	text := string(content.Data)
	err := clipboard.WriteAll(text)
	if err != nil {
		return fmt.Errorf("failed to write clipboard: %w", err)
	}

	c.logger.Debug("Wrote clipboard content",
		zap.String("type", string(content.Type)),
		zap.Int("size", len(content.Data)))

	return nil
}

// Close releases resources
func (c *LinuxClipboard) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isRunning {
		c.cancel()
		c.isRunning = false
	}
}

// NewDaemonizer creates a new Linux daemonizer instance
func NewDaemonizer() Daemonizer {
	return &LinuxDaemonizer{}
}

// LinuxDaemonizer implements the Daemonizer interface for Linux
type LinuxDaemonizer struct{}

func (d *LinuxDaemonizer) Daemonize(executable string, args []string, workDir string, dataDir string) (int, error) {
	// TODO: Implement Linux daemonization
	return 0, fmt.Errorf("daemonization not implemented yet")
}

func (d *LinuxDaemonizer) IsRunningAsDaemon() bool {
	// TODO: Implement daemon check
	return false
} 