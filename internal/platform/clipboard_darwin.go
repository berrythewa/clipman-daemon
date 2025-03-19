//go:build darwin
// +build darwin

package platform

import (
	"bytes"
	"fmt"
	"image"
	"image/png"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/types"
	zyeclip "github.com/zyedidia/clipboard"
)

// DarwinClipboard is the macOS-specific clipboard implementation
type DarwinClipboard struct {
	lastChangeCount uint64
}

// NewClipboard creates a new platform-specific clipboard implementation
func NewClipboard() *DarwinClipboard {
	// Initialize with current change count
	return &DarwinClipboard{
		lastChangeCount: getChangeCount(),
	}
}

// getChangeCount gets the current change count from the pasteboard
func getChangeCount() uint64 {
	// Use external C function to get the change count
	// This is implemented via CGO in clipboard_darwin.c
	return 0 // Placeholder - would call C.getChangeCount() in actual implementation
}

// Read gets the current clipboard content
func (c *DarwinClipboard) Read() (*types.ClipboardContent, error) {
	if zyeclip.Unsupported {
		return nil, fmt.Errorf("clipboard is unsupported on this system")
	}

	// Check if the clipboard has changed
	currentChangeCount := getChangeCount()
	if currentChangeCount == c.lastChangeCount {
		return nil, fmt.Errorf("content unchanged")
	}
	c.lastChangeCount = currentChangeCount

	text, err := zyeclip.Read(zyeclip.FmtText)
	if err == nil {
		return &types.ClipboardContent{
			Type:    types.TypeText,
			Data:    []byte(text),
			Created: time.Now(),
		}, nil
	}

	img, err := zyeclip.Read(zyeclip.FmtImage)
	if err == nil {
		// Convert image to PNG
		buf := new(bytes.Buffer)
		if err := png.Encode(buf, img.(image.Image)); err != nil {
			return nil, fmt.Errorf("failed to encode image: %v", err)
		}
		return &types.ClipboardContent{
			Type:    types.TypeImage,
			Data:    buf.Bytes(),
			Created: time.Now(),
		}, nil
	}

	return nil, fmt.Errorf("no supported content in clipboard")
}

// MonitorChanges monitors for clipboard changes and sends updates to the channel
func (c *DarwinClipboard) MonitorChanges(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	
	go func() {
		for {
			select {
			case <-stopCh:
				return
			case <-ticker.C:
				// Check if the clipboard has changed using the change count
				currentChangeCount := getChangeCount()
				if currentChangeCount != c.lastChangeCount {
					// Update the change count first to avoid duplicate reads
					c.lastChangeCount = currentChangeCount
					
					// Attempt to read the clipboard content
					content, err := c.Read()
					if err != nil {
						// Skip errors due to unchanged content (already handled)
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
		}
	}()
}

// Write sets the clipboard content
func (c *DarwinClipboard) Write(content *types.ClipboardContent) error {
	if zyeclip.Unsupported {
		return fmt.Errorf("clipboard is unsupported on this system")
	}

	switch content.Type {
	case types.TypeText:
		return zyeclip.Write(zyeclip.FmtText, string(content.Data))
	default:
		return fmt.Errorf("unsupported content type: %s", content.Type)
	}
}

// Close cleans up any resources
func (c *DarwinClipboard) Close() {
	// Nothing to clean up for Darwin clipboard
}