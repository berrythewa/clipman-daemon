package clipboard

import (
	"fmt"
	"time"

	"github.com/atotto/clipboard"
)

type ContentType string

const (
	TypeText ContentType = "text"
)

type ClipboardContent struct {
	Type    ContentType
	Data    string
	Created time.Time
}

type Clipboard interface {
	Read() (*ClipboardContent, error)
	Write(*ClipboardContent) error
}

type AtottoClipboard struct{}

func NewClipboard() Clipboard {
	return &AtottoClipboard{}
}

func (c *AtottoClipboard) Read() (*ClipboardContent, error) {
	text, err := clipboard.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("failed to read clipboard: %v", err)
	}

	return &ClipboardContent{
		Type:    TypeText,
		Data:    text,
		Created: time.Now(),
	}, nil
}

func (c *AtottoClipboard) Write(content *ClipboardContent) error {
	if content.Type != TypeText {
		return fmt.Errorf("only text content is supported")
	}

	return clipboard.WriteAll(content.Data)
}
