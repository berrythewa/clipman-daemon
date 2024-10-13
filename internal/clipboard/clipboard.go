package clipboard

import (
	"fmt"
	"time"
	
	"net/http"

	atottoClip "github.com/atotto/clipboard"
    "github.com/berrythewa/clipman-daemon/internal/types"

)

type Clipboard interface {
    Read() (*types.ClipboardContent, error)
    Write(*types.ClipboardContent) error
}

type AtottoClipboard struct{}

func NewClipboard() Clipboard {
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


// func (c1 *types.ClipboardContent) Equal(c2 *types.ClipboardContent) bool {
// 	if c1 == nil || c2 == nil {
// 		return c1 == c2
// 	}
// 	return c1.Type == c2.Type && bytes.Equal(c1.Data, c2.Data)
// }