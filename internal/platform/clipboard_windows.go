//go:build windows
// +build windows

package platform

/*
TODO: Clipboard Format Implementation Summary

- [ ] Image writing support (currently only reading is implemented)
- [ ] File writing support (currently only reading is implemented)
- [ ] HTML content support
- [ ] RTF (Rich Text Format) support
- [ ] Performance optimization for large clipboard content
- [ ] Content caching to improve performance
- [ ] Custom format registration and handling
- [ ] Implement proper unit tests for all clipboard operations
- [ ] Support for CF_BITMAP and CF_ENHMETAFILE formats
- [ ] Implement better conversion between different image formats
*/

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"regexp"
	"sync"
	"time"
	"unsafe"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"golang.org/x/sys/windows"
)

// ClipboardLogger defines the interface for clipboard logging
type ClipboardLogger interface {
	Printf(format string, v ...interface{})
}

// DefaultLogger provides a basic implementation of the ClipboardLogger
type DefaultLogger struct{}

// Printf implements the ClipboardLogger interface
func (l DefaultLogger) Printf(format string, v ...interface{}) {
	fmt.Printf(format+"\n", v...)
}

// WindowsClipboard is the Windows-specific clipboard implementation
type WindowsClipboard struct {
	hwnd           windows.Handle
	lastClipFormat uint32
	msgChan        chan uint32
	logger         ClipboardLogger
	mu             sync.Mutex
	isRunning      bool
	windowThread   chan struct{}
}

// Window class name for the clipboard listener
const clipboardListenerClass = "ClipmanClipboardListener"

// Windows message constants
const (
	WM_CLIPBOARDUPDATE = 0x031D
	WM_DESTROY         = 0x0002
	WM_CLOSE           = 0x0010
)

// Format priority order to try when reading from clipboard
var formatPriorities = []uint32{
	windows.CF_HDROP,       // Files first
	windows.CF_DIBV5,       // Best image format
	windows.CF_DIB,         // Fallback image format
	windows.CF_UNICODETEXT, // Best text format
	windows.CF_HTML,        // Rich text formats
	windows.CF_TEXT,        // Fallback text formats
	windows.CF_OEMTEXT,
}

// Format names for logging
var formatNames = map[uint32]string{
	windows.CF_UNICODETEXT: "CF_UNICODETEXT",
	windows.CF_TEXT:        "CF_TEXT",
	windows.CF_OEMTEXT:     "CF_OEMTEXT",
	windows.CF_DIB:         "CF_DIB",
	windows.CF_DIBV5:       "CF_DIBV5",
	windows.CF_BITMAP:      "CF_BITMAP",
	windows.CF_HDROP:       "CF_HDROP",
	windows.CF_HTML:        "CF_HTML",
	windows.CF_LOCALE:      "CF_LOCALE",
	windows.CF_ENHMETAFILE: "CF_ENHMETAFILE",
}

// NewClipboard creates a new platform-specific clipboard implementation
func NewClipboard() *WindowsClipboard {
	c := &WindowsClipboard{
		msgChan:      make(chan uint32, 10),
		logger:       &DefaultLogger{},
		windowThread: make(chan struct{}),
	}
	
	// Create window in a separate goroutine
	go c.createClipboardWindow()
	
	return c
}

// SetLogger sets a custom logger
func (c *WindowsClipboard) SetLogger(logger ClipboardLogger) {
	c.logger = logger
}

// createClipboardWindow creates a window to receive clipboard notifications
func (c *WindowsClipboard) createClipboardWindow() {
	defer close(c.windowThread)
	
	c.logger.Printf("Creating clipboard listener window")
	
	// Register window class
	className, err := windows.UTF16PtrFromString(clipboardListenerClass)
	if err != nil {
		c.logger.Printf("Error creating UTF16 class name: %v", err)
		return
	}
	
	wndClass := windows.WNDCLASSEX{
		CbSize:        uint32(unsafe.Sizeof(windows.WNDCLASSEX{})),
		LpfnWndProc:   windows.NewCallback(c.wndProc),
		HInstance:     windows.Handle(windows.GetModuleHandle(nil)),
		LpszClassName: className,
	}
	
	atom, err := windows.RegisterClassEx(&wndClass)
	if err != nil {
		c.logger.Printf("Error registering window class: %v", err)
		return
	}
	c.logger.Printf("Window class registered with atom: %d", atom)
	
	// Create window
	hwnd, err := windows.CreateWindowEx(
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
	
	if err != nil {
		c.logger.Printf("Error creating window: %v", err)
		return
	}
	
	c.mu.Lock()
	c.hwnd = hwnd
	c.isRunning = true
	c.mu.Unlock()
	
	c.logger.Printf("Clipboard listener window created: %v", hwnd)
	
	// Register for clipboard updates
	if err := windows.AddClipboardFormatListener(c.hwnd); err != nil {
		c.logger.Printf("Error adding clipboard format listener: %v", err)
		windows.DestroyWindow(c.hwnd)
		return
	}
	
	c.logger.Printf("Clipboard format listener registered")
	
	// Message loop
	var msg windows.MSG
	for {
		result, err := windows.GetMessage(&msg, 0, 0, 0)
		if err != nil {
			c.logger.Printf("Error getting message: %v", err)
			break
		}
		
		if result == 0 {
			// WM_QUIT message
			c.logger.Printf("Received WM_QUIT, exiting message loop")
			break
		}
		
		windows.TranslateMessage(&msg)
		windows.DispatchMessage(&msg)
	}
	
	c.mu.Lock()
	c.isRunning = false
	c.mu.Unlock()
	
	c.logger.Printf("Clipboard listener window thread exited")
}

// wndProc is the window procedure callback
func (c *WindowsClipboard) wndProc(hwnd windows.Handle, msg uint32, wparam, lparam uintptr) uintptr {
	switch msg {
	case WM_CLIPBOARDUPDATE:
		// Get formats for logging
		var formats []string
		for format := uint32(0); format < 0xFFFF; format++ {
			if windows.IsClipboardFormatAvailable(format) {
				if name, ok := formatNames[format]; ok {
					formats = append(formats, name)
				} else {
					formats = append(formats, fmt.Sprintf("0x%X", format))
				}
			}
		}
		
		c.logger.Printf("WM_CLIPBOARDUPDATE received, available formats: %v", formats)
		
		// Send notification through channel
		select {
		case c.msgChan <- msg:
			c.logger.Printf("Clipboard update message sent to channel")
		default:
			c.logger.Printf("Channel full, clipboard update message dropped")
		}
		return 0
		
	case WM_DESTROY, WM_CLOSE:
		c.logger.Printf("Window closing, posting quit message")
		windows.PostQuitMessage(0)
		return 0
	}
	
	return windows.DefWindowProc(hwnd, msg, wparam, lparam)
}

// detectBestFormat checks available formats and chooses the highest priority one
func (c *WindowsClipboard) detectBestFormat() uint32 {
	for _, format := range formatPriorities {
		if windows.IsClipboardFormatAvailable(format) {
			formatName := "Unknown"
			if name, ok := formatNames[format]; ok {
				formatName = name
			}
			c.logger.Printf("Selected best available format: %s (0x%X)", formatName, format)
			return format
		}
	}
	
	c.logger.Printf("No supported clipboard format available")
	return 0 // No supported format available
}

// Read gets the current clipboard content
func (c *WindowsClipboard) Read() (*types.ClipboardContent, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.logger.Printf("Reading clipboard content")
	
	err := windows.OpenClipboard(0)
	if err != nil {
		return nil, fmt.Errorf("failed to open clipboard: %v", err)
	}
	defer windows.CloseClipboard()
	
	// Get current clipboard sequence number
	format := windows.GetClipboardSequenceNumber()
	if format == c.lastClipFormat {
		c.logger.Printf("Clipboard content unchanged (sequence: %d)", format)
		return nil, fmt.Errorf("content unchanged")
	}
	c.lastClipFormat = format
	c.logger.Printf("New clipboard content detected (sequence: %d)", format)
	
	// Detect best available format
	bestFormat := c.detectBestFormat()
	if bestFormat == 0 {
		return nil, fmt.Errorf("no supported clipboard format available")
	}
	
	// Read content based on detected format
	content, err := c.readFormat(bestFormat)
	if err != nil {
		return nil, err
	}
	
	c.logger.Printf("Read clipboard content: type=%s, size=%d bytes", 
		content.Type, len(content.Data))
	
	return content, nil
}

// readFormat handles reading different clipboard formats
func (c *WindowsClipboard) readFormat(format uint32) (*types.ClipboardContent, error) {
	switch format {
	case windows.CF_UNICODETEXT, windows.CF_TEXT, windows.CF_OEMTEXT:
		return c.readTextFormat(format)
	case windows.CF_DIB, windows.CF_DIBV5:
		return c.readImageFormat(format)
	case windows.CF_HDROP:
		return c.readFileFormat()
	default:
		return nil, fmt.Errorf("unsupported format: %d", format)
	}
}

// readTextFormat reads text content from clipboard
func (c *WindowsClipboard) readTextFormat(format uint32) (*types.ClipboardContent, error) {
	h, err := windows.GetClipboardData(format)
	if err != nil {
		return nil, fmt.Errorf("failed to get clipboard data: %v", err)
	}
	
	var textData string
	
	if format == windows.CF_UNICODETEXT {
		// Unicode text
		textPtr := (*uint16)(unsafe.Pointer(h))
		textData, err = windows.UTF16PtrToString(textPtr)
		if err != nil {
			return nil, fmt.Errorf("failed to convert Unicode text: %v", err)
		}
	} else {
		// ANSI or OEM text
		ptr, err := windows.GlobalLock(h)
		if err != nil {
			return nil, fmt.Errorf("failed to lock global memory: %v", err)
		}
		defer windows.GlobalUnlock(h)
		
		// Find null terminator
		p := ptr
		for {
			b := *(*byte)(unsafe.Pointer(p))
			if b == 0 {
				break
			}
			p++
		}
		
		// Calculate length and convert to string
		length := p - ptr
		bytes := make([]byte, length)
		for i := uintptr(0); i < length; i++ {
			bytes[i] = *(*byte)(unsafe.Pointer(ptr + i))
		}
		
		textData = string(bytes)
	}
	
	// Check if text is a URL
	if isURL(textData) {
		c.logger.Printf("Detected URL in clipboard: %s", truncateString(textData, 100))
		return &types.ClipboardContent{
			Type:    types.TypeURL,
			Data:    []byte(textData),
			Created: time.Now(),
		}, nil
	}
	
	// Regular text
	c.logger.Printf("Detected text in clipboard: %s", truncateString(textData, 100))
	return &types.ClipboardContent{
		Type:    types.TypeText,
		Data:    []byte(textData),
		Created: time.Now(),
	}, nil
}

// readImageFormat reads image content from clipboard
func (c *WindowsClipboard) readImageFormat(format uint32) (*types.ClipboardContent, error) {
	h, err := windows.GetClipboardData(format)
	if err != nil {
		return nil, fmt.Errorf("failed to get image data: %v", err)
	}
	
	// Lock the memory to access the bitmap data
	ptr, err := windows.GlobalLock(h)
	if err != nil {
		return nil, fmt.Errorf("failed to lock memory: %v", err)
	}
	defer windows.GlobalUnlock(h)
	
	// Convert DIB to PNG
	c.logger.Printf("Converting DIB image data to PNG")
	imgData, err := c.convertDIBtoPNG(ptr, format)
	if err != nil {
		return nil, fmt.Errorf("failed to convert image: %v", err)
	}
	
	c.logger.Printf("Converted image size: %d bytes", len(imgData))
	return &types.ClipboardContent{
		Type:    types.TypeImage,
		Data:    imgData,
		Created: time.Now(),
	}, nil
}

// readFileFormat reads file list from clipboard
func (c *WindowsClipboard) readFileFormat() (*types.ClipboardContent, error) {
	h, err := windows.GetClipboardData(windows.CF_HDROP)
	if err != nil {
		return nil, fmt.Errorf("failed to get file list: %v", err)
	}
	
	// Get count of files in the drop
	count := windows.DragQueryFile(h, 0xFFFFFFFF, nil, 0)
	c.logger.Printf("File drop contains %d files", count)
	
	// If there's just one file, treat it as FilePath
	if count == 1 {
		// Buffer for filename
		buf := make([]uint16, windows.MAX_PATH)
		windows.DragQueryFile(h, 0, &buf[0], windows.MAX_PATH)
		filePath := windows.UTF16ToString(buf)
		
		c.logger.Printf("Single file path detected: %s", filePath)
		return &types.ClipboardContent{
			Type:    types.TypeFilePath,
			Data:    []byte(filePath),
			Created: time.Now(),
		}, nil
	} else {
		// Multiple files - serialize as JSON
		files := make([]string, 0, count)
		for i := uint32(0); i < count; i++ {
			buf := make([]uint16, windows.MAX_PATH)
			windows.DragQueryFile(h, i, &buf[0], windows.MAX_PATH)
			filePath := windows.UTF16ToString(buf)
			files = append(files, filePath)
		}
		
		// Serialize the file list
		fileData, err := json.Marshal(files)
		if err != nil {
			return nil, fmt.Errorf("failed to serialize file list: %v", err)
		}
		
		c.logger.Printf("Multiple files detected, json size: %d bytes", len(fileData))
		return &types.ClipboardContent{
			Type:    types.TypeFile,
			Data:    fileData,
			Created: time.Now(),
		}, nil
	}
}

// convertDIBtoPNG converts DIB image data to PNG format
func (c *WindowsClipboard) convertDIBtoPNG(dibPtr uintptr, format uint32) ([]byte, error) {
	// Parse DIB header
	header := (*windows.BITMAPINFOHEADER)(unsafe.Pointer(dibPtr))
	
	width := int(header.BiWidth)
	height := int(header.BiHeight)
	bitCount := int(header.BiBitCount)
	
	c.logger.Printf("DIB image: %dx%d pixels, %d bits per pixel", width, height, bitCount)
	
	// Only support common bit depths
	if bitCount != 24 && bitCount != 32 {
		return nil, fmt.Errorf("unsupported bit depth: %d", bitCount)
	}
	
	// Create a new RGBA image
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	
	// Calculate stride (bytes per row, aligned to 4-byte boundary)
	stride := (width*bitCount + 31) / 32 * 4
	
	// DIB data follows the header (and color table for <24 bpp, but we don't support those)
	pixelDataOffset := uintptr(unsafe.Sizeof(*header))
	if bitCount <= 8 {
		// Color table size for <=8bpp: 2^bitCount entries, 4 bytes each
		pixelDataOffset += uintptr(1<<bitCount) * 4
	}
	
	pixelData := dibPtr + pixelDataOffset
	
	// DIBs are stored bottom-up by default
	isBottomUp := height > 0
	absHeight := height
	if !isBottomUp {
		absHeight = -height
	}
	
	// Copy pixel data
	for y := 0; y < absHeight; y++ {
		srcY := y
		if isBottomUp {
			srcY = absHeight - y - 1
		}
		
		srcRow := pixelData + uintptr(srcY*stride)
		
		for x := 0; x < width; x++ {
			var offset uintptr
			var r, g, b, a uint8
			
			if bitCount == 24 {
				// 24-bit format: BGR (no alpha)
				offset = uintptr(x * 3)
				b = *(*uint8)(unsafe.Pointer(srcRow + offset))
				g = *(*uint8)(unsafe.Pointer(srcRow + offset + 1))
				r = *(*uint8)(unsafe.Pointer(srcRow + offset + 2))
				a = 255 // Fully opaque
			} else if bitCount == 32 {
				// 32-bit format: BGRA
				offset = uintptr(x * 4)
				b = *(*uint8)(unsafe.Pointer(srcRow + offset))
				g = *(*uint8)(unsafe.Pointer(srcRow + offset + 1))
				r = *(*uint8)(unsafe.Pointer(srcRow + offset + 2))
				a = *(*uint8)(unsafe.Pointer(srcRow + offset + 3))
			}
			
			img.SetRGBA(x, y, image.RGBA{R: r, G: g, B: b, A: a})
		}
	}
	
	// Encode as PNG
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, fmt.Errorf("failed to encode image as PNG: %v", err)
	}
	
	return buf.Bytes(), nil
}

// MonitorChanges monitors for clipboard changes and sends updates to the channel
func (c *WindowsClipboard) MonitorChanges(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
	c.logger.Printf("Starting clipboard monitoring")
	
	go func() {
		for {
			select {
			case <-stopCh:
				c.logger.Printf("Received stop signal, ending clipboard monitoring")
				return
				
			case <-c.msgChan:
				// Clipboard changed, read the new content
				c.logger.Printf("Processing clipboard change notification")
				content, err := c.Read()
				if err != nil {
					if err.Error() == "content unchanged" {
						c.logger.Printf("Content unchanged, skipping")
						continue
					}
					c.logger.Printf("Error reading clipboard: %v", err)
					continue
				}
				
				// Send the content to the channel
				select {
				case contentCh <- content:
					c.logger.Printf("Sent new clipboard content to channel: type=%s, size=%d bytes",
						content.Type, len(content.Data))
				case <-stopCh:
					c.logger.Printf("Received stop signal while sending content")
					return
				}
			}
		}
	}()
	
	// Monitor window health and restart if needed
	go func() {
		for {
			select {
			case <-stopCh:
				return
			case <-c.windowThread:
				// Window thread exited unexpectedly
				c.logger.Printf("Clipboard window thread exited, restarting")
				
				c.mu.Lock()
				isRunning := c.isRunning
				c.mu.Unlock()
				
				if !isRunning {
					c.windowThread = make(chan struct{})
					go c.createClipboardWindow()
					c.logger.Printf("Clipboard window restarted")
				}
			}
		}
	}()
}

// Write sets the clipboard content
func (c *WindowsClipboard) Write(content *types.ClipboardContent) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.logger.Printf("Writing to clipboard: type=%s, size=%d bytes", content.Type, len(content.Data))
	
	switch content.Type {
	case types.TypeText, types.TypeString, types.TypeURL:
		return c.writeTextFormat(content)
	case types.TypeImage:
		return fmt.Errorf("writing image to clipboard is not implemented yet")
	case types.TypeFilePath, types.TypeFile:
		return fmt.Errorf("writing files to clipboard is not implemented yet")
	default:
		return fmt.Errorf("unsupported content type: %s", content.Type)
	}
}

// writeTextFormat writes text content to clipboard
func (c *WindowsClipboard) writeTextFormat(content *types.ClipboardContent) error {
	c.logger.Printf("Writing text to clipboard: %s", truncateString(string(content.Data), 100))
	
	err := windows.OpenClipboard(0)
	if err != nil {
		return fmt.Errorf("failed to open clipboard: %v", err)
	}
	defer windows.CloseClipboard()
	
	err = windows.EmptyClipboard()
	if err != nil {
		return fmt.Errorf("failed to empty clipboard: %v", err)
	}
	
	data, err := windows.UTF16FromString(string(content.Data))
	if err != nil {
		return fmt.Errorf("failed to convert text to UTF16: %v", err)
	}
	
	size := len(data) * int(unsafe.Sizeof(uint16(0)))
	
	h, err := windows.GlobalAlloc(windows.GMEM_MOVEABLE, uint32(size))
	if err != nil {
		return fmt.Errorf("failed to allocate memory: %v", err)
	}
	
	ptr, err := windows.GlobalLock(h)
	if err != nil {
		windows.GlobalFree(h)
		return fmt.Errorf("failed to lock memory: %v", err)
	}
	
	// Copy data to allocated memory
	copy((*[1 << 30]uint16)(unsafe.Pointer(ptr))[:len(data)], data)
	windows.GlobalUnlock(h)
	
	if _, err := windows.SetClipboardData(windows.CF_UNICODETEXT, h); err != nil {
		windows.GlobalFree(h)
		return fmt.Errorf("failed to set clipboard data: %v", err)
	}
	
	c.logger.Printf("Successfully wrote text to clipboard")
	return nil
}

// Close releases any resources
func (c *WindowsClipboard) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.logger.Printf("Closing clipboard resources")
	
	if c.hwnd != 0 {
		c.logger.Printf("Removing clipboard format listener and destroying window")
		windows.RemoveClipboardFormatListener(c.hwnd)
		windows.DestroyWindow(c.hwnd)
		c.hwnd = 0
	}
}

// Helper functions

// isURL checks if a string is a URL
func isURL(text string) bool {
	urlPattern := regexp.MustCompile(`^(https?|ftp)://[^\s/$.?#].[^\s]*$`)
	return urlPattern.MatchString(text)
}

// truncateString truncates a string to the specified length
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}