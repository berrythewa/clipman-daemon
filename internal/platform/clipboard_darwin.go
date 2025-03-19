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
type DarwinClipboard struct{}

// NewClipboard creates a new platform-specific clipboard implementation
func NewClipboard() *DarwinClipboard {
	return &DarwinClipboard{}
}

// Read gets the current clipboard content
func (c *DarwinClipboard) Read() (*types.ClipboardContent, error) {
	if zyeclip.Unsupported {
		return nil, fmt.Errorf("clipboard is unsupported on this system")
	}

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