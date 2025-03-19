//go:build windows
// +build windows

package platform

import (
	"fmt"
	"time"
	"unsafe"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"golang.org/x/sys/windows"
)

// WindowsClipboard is the Windows-specific clipboard implementation
type WindowsClipboard struct{}

// NewClipboard creates a new platform-specific clipboard implementation
func NewClipboard() *WindowsClipboard {
	return &WindowsClipboard{}
}

// Read gets the current clipboard content
func (c *WindowsClipboard) Read() (*types.ClipboardContent, error) {
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

	return &types.ClipboardContent{
		Type:    types.TypeText,
		Data:    []byte(data),
		Created: time.Now(),
	}, nil
}

// Write sets the clipboard content
func (c *WindowsClipboard) Write(content *types.ClipboardContent) error {
	if content.Type != types.TypeText {
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

	data := windows.UTF16FromString(string(content.Data))
	size := len(data) * int(windows.SizeofUint16)

	h, err := windows.GlobalAlloc(windows.GMEM_MOVEABLE, uint32(size))
	if err != nil {
		return fmt.Errorf("failed to allocate memory: %v", err)
	}

	ptr, err := windows.GlobalLock(h)
	if err != nil {
		return fmt.Errorf("failed to lock memory: %v", err)
	}

	windows.CopyMemory(ptr, (uintptr)(unsafe.Pointer(&data[0])), uintptr(size))
	windows.GlobalUnlock(h)

	if _, err := windows.SetClipboardData(windows.CF_UNICODETEXT, h); err != nil {
		return fmt.Errorf("failed to set clipboard data: %v", err)
	}

	return nil
}