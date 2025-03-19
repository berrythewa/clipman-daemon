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
type WindowsClipboard struct {
	hwnd           windows.Handle
	lastClipFormat uint32
	msgChan        chan uint32
}

// Window class name for the clipboard listener
const clipboardListenerClass = "ClipmanClipboardListener"

// Windows message constants
const (
	WM_CLIPBOARDUPDATE = 0x031D
	WM_DESTROY         = 0x0002
	WM_CLOSE           = 0x0010
)

// NewClipboard creates a new platform-specific clipboard implementation
func NewClipboard() *WindowsClipboard {
	c := &WindowsClipboard{
		msgChan: make(chan uint32, 10),
	}
	
	// Create window in a separate goroutine
	go c.createClipboardWindow()
	
	return c
}

// createClipboardWindow creates a window to receive clipboard notifications
func (c *WindowsClipboard) createClipboardWindow() {
	// Register window class
	className, _ := windows.UTF16PtrFromString(clipboardListenerClass)
	wndClass := windows.WNDCLASSEX{
		CbSize:    uint32(unsafe.Sizeof(windows.WNDCLASSEX{})),
		LpfnWndProc: windows.NewCallback(c.wndProc),
		HInstance:   windows.Handle(windows.GetModuleHandle(nil)),
		LpszClassName: className,
	}
	
	windows.RegisterClassEx(&wndClass)
	
	// Create window
	hwnd, _ := windows.CreateWindowEx(
		0,
		className,
		nil,
		0,
		0, 0, 0, 0,
		0,
		0,
		windows.Handle(windows.GetModuleHandle(nil)),
		nil,
	)
	
	c.hwnd = hwnd
	
	// Register for clipboard updates
	windows.AddClipboardFormatListener(c.hwnd)
	
	// Message loop
	var msg windows.MSG
	for {
		if windows.GetMessage(&msg, 0, 0, 0) != 0 {
			windows.TranslateMessage(&msg)
			windows.DispatchMessage(&msg)
		}
	}
}

// wndProc is the window procedure callback
func (c *WindowsClipboard) wndProc(hwnd windows.Handle, msg uint32, wparam, lparam uintptr) uintptr {
	switch msg {
	case WM_CLIPBOARDUPDATE:
		// Send notification through channel
		select {
		case c.msgChan <- msg:
		default:
			// Channel full, drop message
		}
		return 0
	case WM_DESTROY, WM_CLOSE:
		windows.PostQuitMessage(0)
		return 0
	}
	return windows.DefWindowProc(hwnd, msg, wparam, lparam)
}

// Read gets the current clipboard content
func (c *WindowsClipboard) Read() (*types.ClipboardContent, error) {
	err := windows.OpenClipboard(0)
	if err != nil {
		return nil, fmt.Errorf("failed to open clipboard: %v", err)
	}
	defer windows.CloseClipboard()
	
	// Get current clipboard format
	format := windows.GetClipboardSequenceNumber()
	if format == c.lastClipFormat {
		return nil, fmt.Errorf("content unchanged")
	}
	c.lastClipFormat = format

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

// MonitorChanges monitors for clipboard changes and sends updates to the channel
func (c *WindowsClipboard) MonitorChanges(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
	go func() {
		for {
			select {
			case <-stopCh:
				return
			case <-c.msgChan:
				// Clipboard changed, read the new content
				content, err := c.Read()
				if err != nil {
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
	}()
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

// Close releases any resources
func (c *WindowsClipboard) Close() {
	if c.hwnd != 0 {
		windows.RemoveClipboardFormatListener(c.hwnd)
		windows.DestroyWindow(c.hwnd)
		c.hwnd = 0
	}
}