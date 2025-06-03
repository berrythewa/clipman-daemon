//go:build linux
// +build linux

package platform

import (
	"context"
	"fmt"
	"sync"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"go.uber.org/zap"
	"github.com/BurntSushi/xgb"
	"github.com/BurntSushi/xgb/xproto"
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

// LinuxClipboard provides a Linux clipboard abstraction using X11
type LinuxClipboard struct {
	mu          sync.Mutex
	conn        *xgb.Conn
	window      xproto.Window
	lastContent string
	isRunning   bool
	logger      *zap.Logger
	ctx         context.Context
	cancel      context.CancelFunc
}

// NewClipboard creates a new LinuxClipboard instance
func NewClipboard(logger *zap.Logger) Clipboard {
	if logger == nil {
		logger = zap.NewNop()
	}

	ctx, cancel := context.WithCancel(context.Background())
	return &LinuxClipboard{
		logger: logger,
		ctx:    ctx,
		cancel: cancel,
	}
}

// Read returns the current clipboard content
func (c *LinuxClipboard) Read() (*types.ClipboardContent, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.conn == nil {
		if err := c.connect(); err != nil {
			return nil, fmt.Errorf("failed to connect to X server: %w", err)
		}
	}

	// Get the current selection owner
	owner, err := xproto.GetSelectionOwner(c.conn, xproto.AtomPrimary).Reply()
	if err != nil {
		return nil, fmt.Errorf("failed to get selection owner: %w", err)
	}

	if owner.Owner == 0 {
		return nil, fmt.Errorf("no selection owner")
	}

	// Convert the selection to a string
	err = xproto.ConvertSelection(
		c.conn,
		c.window,
		xproto.AtomPrimary,
		xproto.AtomString,
		xproto.AtomPrimary,
		xproto.TimeCurrentTime,
	).Check()
	if err != nil {
		return nil, fmt.Errorf("failed to convert selection: %w", err)
	}

	// Get the property
	prop, err := xproto.GetProperty(
		c.conn,
		true,
		c.window,
		xproto.AtomPrimary,
		xproto.AtomAny,
		0,
		(1<<32)-1,
	).Reply()
	if err != nil {
		return nil, fmt.Errorf("failed to get property: %w", err)
	}

	// Create content from the property
	content := &types.ClipboardContent{
		Type: types.TypeText,
		Data: prop.Value,
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

	if c.conn == nil {
		if err := c.connect(); err != nil {
			return fmt.Errorf("failed to connect to X server: %w", err)
		}
	}

	// Set the selection owner
	err := xproto.SetSelectionOwner(
		c.conn,
		c.window,
		xproto.AtomPrimary,
		xproto.TimeCurrentTime,
	).Check()
	if err != nil {
		return fmt.Errorf("failed to set selection owner: %w", err)
	}

	// Store the content
	c.lastContent = string(content.Data)

	c.logger.Debug("Wrote clipboard content",
		zap.String("type", string(content.Type)),
		zap.Int("size", len(content.Data)))

	return nil
}

// MonitorChanges starts monitoring for clipboard changes
func (c *LinuxClipboard) MonitorChanges(handler ClipboardChangeHandler) {
	c.mu.Lock()
	if c.isRunning {
		c.mu.Unlock()
		return
	}
	c.isRunning = true
	c.mu.Unlock()

	if err := c.connect(); err != nil {
		c.logger.Error("Failed to connect to X server", zap.Error(err))
		return
	}

	// Set up event mask
	err := xproto.ChangeWindowAttributes(
		c.conn,
		c.window,
		xproto.CwEventMask,
		[]uint32{xproto.EventMaskPropertyChange},
	).Check()
	if err != nil {
		c.logger.Error("Failed to set event mask", zap.Error(err))
		return
	}

	// Start monitoring loop
	go func() {
		defer func() {
			c.mu.Lock()
			c.isRunning = false
			c.mu.Unlock()
		}()

		for {
			select {
			case <-c.ctx.Done():
				return
			default:
				// Check for events
				ev, err := c.conn.WaitForEvent()
				if err != nil {
					c.logger.Error("Error waiting for event", zap.Error(err))
					continue
				}

				// Handle property change events
				if prop, ok := ev.(xproto.PropertyNotifyEvent); ok {
					if prop.Atom == xproto.AtomPrimary {
						content, err := c.Read()
						if err != nil {
							c.logger.Error("Failed to read clipboard", zap.Error(err))
							continue
						}

						handler(content)
					}
				}
			}
		}
	}()
}

// Close releases resources
func (c *LinuxClipboard) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.isRunning {
		c.cancel()
		c.isRunning = false
	}

	if c.conn != nil {
		c.conn.Close()
		c.conn = nil
	}
}

// connect establishes a connection to the X server
func (c *LinuxClipboard) connect() error {
	conn, err := xgb.NewConn()
	if err != nil {
		return fmt.Errorf("failed to connect to X server: %w", err)
	}

	// Create a window to receive events
	screen := xproto.Setup(conn).DefaultScreen(conn)
	window, err := xproto.NewWindowId(conn)
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to create window: %w", err)
	}

	err = xproto.CreateWindow(
		conn,
		screen.RootDepth,
		window,
		screen.Root,
		0, 0,
		1, 1,
		0,
		xproto.WindowClassInputOutput,
		screen.RootVisual,
		0,
		nil,
	).Check()
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to create window: %w", err)
	}

	c.conn = conn
	c.window = window
	return nil
} 