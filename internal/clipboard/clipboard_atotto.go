//go:build !darwin
// +build !darwin

package clipboard

import (
	"fmt"
	"time"

	atottoClip "github.com/atotto/clipboard"
	"github.com/berrythewa/clipman-daemon/internal/types"
)

// AtottoClipboard is a fallback clipboard implementation using the atotto/clipboard library
// It only supports text content
type AtottoClipboard struct{}

// NewAtottoClipboard returns a new Atotto-based clipboard implementation
func NewAtottoClipboard() *AtottoClipboard {
	return &AtottoClipboard{}
}

func (c *AtottoClipboard) Read() (*types.ClipboardContent, error) {
	text, err := atottoClip.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read clipboard: %v", err)
	}
	content := &types.ClipboardContent{
		Data:    []byte(text),
		Created: time.Now(),
	}
	content.Type = detectContentType(content.Data)
	return content, nil
}

func (c *AtottoClipboard) Write(content *types.ClipboardContent) error {
	if content.Type != types.TypeText {
		return fmt.Errorf("only text content is supported for writing")
	}
	return atottoClip.WriteAll(string(content.Data))
}
