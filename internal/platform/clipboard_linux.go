package platform

import (
	"fmt"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/clipboard"
	"github.com/atotto/clipboard"
)

type LinuxClipboard struct{}

func NewClipboard() clipboard.Clipboard {
	return &LinuxClipboard{}
}

func (c *LinuxClipboard) Read() (*clipboard.ClipboardContent, error) {
	text, err := clipboard.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read clipboard: %v", err)
	}

	return &clipboard.ClipboardContent{
		Type:    clipboard.TypeText,
		Data:    []byte(text),
		Created: time.Now(),
	}, nil
}

func (c *LinuxClipboard) Write(content *clipboard.ClipboardContent) error {
	if content.Type != clipboard.TypeText {
		return fmt.Errorf("only text content is supported on Linux")
	}

	return clipboard.WriteAll(string(content.Data))
}
