//go:build linux
// +build linux

package platform

import (
	"fmt"
	"time"

	cliplib "github.com/atotto/clipboard"
	"github.com/berrythewa/clipman-daemon/internal/types"
)

// LinuxClipboard is the Linux-specific clipboard implementation
type LinuxClipboard struct{}

// NewClipboard creates a new platform-specific clipboard implementation
func NewClipboard() *LinuxClipboard {
	return &LinuxClipboard{}
}

// Read gets the current clipboard content
func (c *LinuxClipboard) Read() (*types.ClipboardContent, error) {
	text, err := cliplib.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read clipboard: %v", err)
	}

	return &types.ClipboardContent{
		Type:    types.TypeText,
		Data:    []byte(text),
		Created: time.Now(),
	}, nil
}

// Write sets the clipboard content
func (c *LinuxClipboard) Write(content *types.ClipboardContent) error {
	if content.Type != types.TypeText {
		return fmt.Errorf("only text content is supported on Linux")
	}

	return cliplib.WriteAll(string(content.Data))
}
