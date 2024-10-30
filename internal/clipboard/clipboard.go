package clipboard

import (
	"runtime"

	"github.com/berrythewa/clipman-daemon/internal/types"
)

type Clipboard interface {
	Read() (*types.ClipboardContent, error)
	Write(*types.ClipboardContent) error
}

func NewClipboard() Clipboard {
	switch runtime.GOOS {
	case "darwin":
		return NewDarwinClipboard()
	default:
		return NewAtottoClipboard()
	}
}
