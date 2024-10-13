package platform

import (
	"fmt"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/clipboard"
	"golang.org/x/sys/windows"
)

type WindowsClipboard struct{}

func NewClipboard() clipboard.Clipboard {
	return &WindowsClipboard{}
}

func (c *WindowsClipboard) Read() (*clipboard.ClipboardContent, error) {
	err := windows.OpenClipboard(0)
	if err != nil {
		return nil, fmt.Errorf("failed to open clipboard: %v", err)
	}
	defer windows.CloseClipboard()

	h, err := windows.GetClipboardData(windows.CF_UNICODETEXT)
	if err != nil {
		return nil, fmt.Errorf("failed to get clipboard data: %v", err)
	}

	data, err := windows.UTF16PtrToString((*uint16)(h))
	if err != nil {
		return nil, fmt.Errorf("failed to convert clipboard data: %v", err)
	}

	return &clipboard.ClipboardContent{
		Type:    clipboard.TypeText,
		Data:    []byte(data),
		Created: time.Now(),
	}, nil
}

func (c *WindowsClipboard) Write(content *clipboard.ClipboardContent) error {
	if content.Type != clipboard.TypeText {
		return fmt.Errorf("only text content is supported on Windows")
	}

	err := windows.OpenClipboard(0)
	if err != nil {
		return fmt.Errorf("failed to open clipboard: %v", err)
	}
	defer windows.CloseClipboard()

	err = windows.EmptyClipboard()
	if err != nil {
		return fmt.Errorf("failed to empty clipboard: %v", err)
	}

	utf16, err := windows.UTF16FromString(string(content.Data))
	if err != nil {
		return fmt.Errorf("failed to convert string to UTF16: %v", err)
	}

	h, err := windows.GlobalAlloc(windows.GMEM_MOVEABLE, uint32(len(utf16)*2))
	if err != nil {
		return fmt.Errorf("failed to allocate memory: %v", err)
	}

	ptr, err := windows.GlobalLock(h)
	if err != nil {
		return fmt.Errorf("failed to lock memory: %v", err)
	}
	defer windows.GlobalUnlock(h)

	windows.Copy((*[1 << 30]uint16)(ptr)[:len(utf16)], utf16)

	if _, err := windows.SetClipboardData(windows.CF_UNICODETEXT, h); err != nil {
		return fmt.Errorf("failed to set clipboard data: %v", err)
	}

	return nil
}