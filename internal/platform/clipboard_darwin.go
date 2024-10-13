package platform

import (
	"bytes"
	"image"
	"image/png"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/clipboard"
	"github.com/zyedidia/clipboard"
)

type DarwinClipboard struct{}

func NewClipboard() clipboard.Clipboard {
	return &DarwinClipboard{}
}

func (c *DarwinClipboard) Read() (*clipboard.ClipboardContent, error) {
	if clipboard.Unsupported {
		return nil, fmt.Errorf("clipboard is unsupported on this system")
	}

	text, err := clipboard.Read(clipboard.FmtText)
	if err == nil {
		return &clipboard.ClipboardContent{
			Type:    clipboard.TypeText,
			Data:    []byte(text),
			Created: time.Now(),
		}, nil
	}

	img, err := clipboard.Read(clipboard.FmtImage)
	if err == nil {
		// Convert image to PNG
		buf := new(bytes.Buffer)
		if err := png.Encode(buf, img.(image.Image)); err != nil {
			return nil, fmt.Errorf("failed to encode image: %v", err)
		}
		return &clipboard.ClipboardContent{
			Type:    clipboard.TypeImage,
			Data:    buf.Bytes(),
			Created: time.Now(),
		}, nil
	}

	return nil, fmt.Errorf("no supported content in clipboard")
}

func (c *DarwinClipboard) Write(content *clipboard.ClipboardContent) error {
	if clipboard.Unsupported {
		return fmt.Errorf("clipboard is unsupported on this system")
	}

	switch content.Type {
	case clipboard.TypeText:
		return clipboard.Write(clipboard.FmtText, content.Data)
	case clipboard.TypeImage:
		img, _, err := image.Decode(bytes.NewReader(content.Data))
		if err != nil {
			return fmt.Errorf("failed to decode image: %v", err)
		}
		return clipboard.Write(clipboard.FmtImage, img)
	default:
		return fmt.Errorf("unsupported content type: %v", content.Type)
	}
}