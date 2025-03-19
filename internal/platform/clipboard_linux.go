//go:build linux
// +build linux

package platform

import (
	"bytes"
	"fmt"
	"time"

	cliplib "github.com/atotto/clipboard"
	"github.com/berrythewa/clipman-daemon/internal/types"
)

// LinuxClipboard is the Linux-specific clipboard implementation
type LinuxClipboard struct {
	lastContent []byte
}

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
	
	// Check if content has changed
	contentBytes := []byte(text)
	if bytes.Equal(contentBytes, c.lastContent) {
		return nil, fmt.Errorf("content unchanged")
	}
	
	// Update the last content
	c.lastContent = make([]byte, len(contentBytes))
	copy(c.lastContent, contentBytes)

	return &types.ClipboardContent{
		Type:    types.TypeText,
		Data:    contentBytes,
		Created: time.Now(),
	}, nil
}

// MonitorChanges monitors for clipboard changes and sends updates to the channel
func (c *LinuxClipboard) MonitorChanges(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
	// We don't have a proper event-based API for Linux clipboard, so use polling
	ticker := time.NewTicker(500 * time.Millisecond)
	
	go func() {
		defer ticker.Stop()
		
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				content, err := c.Read()
				if err != nil {
					// Skip if content hasn't changed
					if err.Error() == "content unchanged" {
						continue
					}
					fmt.Printf("Error reading clipboard: %v\n", err)
					continue
				}
				
				// Send the content to the channel
				select {
				case contentCh <- content:
				case <-stopCh:
					return
				}
			}
		}
	}()
}

// Write sets the clipboard content
func (c *LinuxClipboard) Write(content *types.ClipboardContent) error {
	if content.Type != types.TypeText {
		return fmt.Errorf("only text content is supported on Linux")
	}

	return cliplib.WriteAll(string(content.Data))
}

// Close cleans up any resources
func (c *LinuxClipboard) Close() {
	// Nothing to clean up for Linux clipboard
}
