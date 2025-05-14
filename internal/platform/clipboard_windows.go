//go:build windows
// +build windows

package platform

/*
TODO: Clipboard Format Implementation Summary
- [ ] File writing support (currently only reading is implemented)
- [ ] Custom format registration and handling
- [ ] Implement proper unit tests for all clipboard operations
- [ ] Implement better conversion between different image formats

IMPORTANT: 
- CF_BITMAP and CF_ENHMETAFILE are Windows-specific clipboard formats that use GDI handles (HBITMAP, HENHMETAFILE).
- These handles are only valid in the originating Windows process/session and MUST NEVER be sent across platforms or stored for later use.
- Always convert these handles to raw, portable data (PNG, JPEG, SVG, etc.) before using outside the process or sending to other platforms.
- This file provides stubs and warnings for these formats. Implement conversion helpers as needed.
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
	"go.uber.org/zap"
)

// WindowsClipboard is the Windows-specific clipboard implementation
type WindowsClipboard struct {
	hwnd           windows.Handle
	lastClipFormat uint32
	msgChan        chan uint32
	logger         *zap.Logger
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

// Add CF_HTML constant (register if not present)
var cfHTML uint32

// Add CF_RTF constant (register if not present)
var cfRTF uint32

func init() {
	cf, err := windows.RegisterClipboardFormat(windows.StringToUTF16Ptr("HTML Format"))
	if err == nil {
		cfHTML = cf
	} else {
		cfHTML = 49349 // fallback to known value
	}
	cf, err = windows.RegisterClipboardFormat(windows.StringToUTF16Ptr("Rich Text Format"))
	if err == nil {
		cfRTF = cf
	} else {
		cfRTF = 0 // fallback, but registration should always work
	}
}

// NewClipboard creates a new platform-specific clipboard implementation
func NewClipboard() *WindowsClipboard {
	c := &WindowsClipboard{
		msgChan:      make(chan uint32, 10),
		logger:       zap.NewNop(),
		windowThread: make(chan struct{}),
	}
	
	// Create window in a separate goroutine
	go c.createClipboardWindow()
	
	return c
}

// SetLogger allows setting a custom zap logger
func (c *WindowsClipboard) SetLogger(logger *zap.Logger) {
	if logger != nil {
		c.logger = logger
	}
}

// createClipboardWindow creates a window to receive clipboard notifications
func (c *WindowsClipboard) createClipboardWindow() {
	defer close(c.windowThread)
	
	c.logger.Info("Creating clipboard listener window")
	
	// Register window class
	className, err := windows.UTF16PtrFromString(clipboardListenerClass)
	if err != nil {
		c.logger.Error("Error creating UTF16 class name", zap.Error(err))
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
		c.logger.Error("Error registering window class", zap.Error(err))
		return
	}
	c.logger.Info("Window class registered", zap.Uint16("atom", atom))
	
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
		c.logger.Error("Error creating window", zap.Error(err))
		return
	}
	
	c.mu.Lock()
	c.hwnd = hwnd
	c.isRunning = true
	c.mu.Unlock()
	
	c.logger.Info("Clipboard listener window created", zap.Any("hwnd", hwnd))
	
	// Register for clipboard updates
	if err := windows.AddClipboardFormatListener(c.hwnd); err != nil {
		c.logger.Error("Error adding clipboard format listener", zap.Error(err))
		windows.DestroyWindow(c.hwnd)
		return
	}
	
	c.logger.Info("Clipboard format listener registered")
	
	// Message loop
	var msg windows.MSG
	for {
		result, err := windows.GetMessage(&msg, 0, 0, 0)
		if err != nil {
			c.logger.Error("Error getting message", zap.Error(err))
			break
		}
		
		if result == 0 {
			// WM_QUIT message
			c.logger.Info("Received WM_QUIT, exiting message loop")
			break
		}
		
		windows.TranslateMessage(&msg)
		windows.DispatchMessage(&msg)
	}
	
	c.mu.Lock()
	c.isRunning = false
	c.mu.Unlock()
	
	c.logger.Info("Clipboard listener window thread exited")
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
		
		c.logger.Info("WM_CLIPBOARDUPDATE received", zap.Strings("formats", formats))
		
		// Send notification through channel
		select {
		case c.msgChan <- msg:
			c.logger.Debug("Clipboard update message sent to channel")
		default:
			c.logger.Warn("Channel full, clipboard update message dropped")
		}
		return 0
		
	case WM_DESTROY, WM_CLOSE:
		c.logger.Info("Window closing, posting quit message")
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
			c.logger.Info("Selected best available format", zap.String("format", formatName), zap.Uint32("format_value", format))
			return format
		}
	}
	
	c.logger.Info("No supported clipboard format available")
	return 0 // No supported format available
}

// Read gets the current clipboard content
func (c *WindowsClipboard) Read() (*types.ClipboardContent, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.logger.Info("Reading clipboard content")
	
	err := windows.OpenClipboard(0)
	if err != nil {
		return nil, fmt.Errorf("failed to open clipboard: %v", err)
	}
	defer windows.CloseClipboard()
	
	// Get current clipboard sequence number
	format := windows.GetClipboardSequenceNumber()
	if format == c.lastClipFormat {
		c.logger.Info("Clipboard content unchanged", zap.Uint32("sequence", format))
		return nil, fmt.Errorf("content unchanged")
	}
	c.lastClipFormat = format
	c.logger.Info("New clipboard content detected", zap.Uint32("sequence", format))
	
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
	
	c.logger.Info("Read clipboard content", zap.String("type", content.Type), zap.Int("size", len(content.Data)))
	
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
	case cfHTML:
		return c.readHTMLFormat()
	case cfRTF:
		return c.readRTFFormat()
	case windows.CF_BITMAP:
		return c.readBitmapHandleFormat()
	case windows.CF_ENHMETAFILE:
		return c.readEnhMetafileFormat()
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
		c.logger.Info("Detected URL in clipboard", zap.String("url", truncateString(textData, 100)))
		return &types.ClipboardContent{
			Type:    types.TypeURL,
			Data:    []byte(textData),
			Created: time.Now(),
		}, nil
	}
	
	// Regular text
	c.logger.Info("Detected text in clipboard", zap.String("text", truncateString(textData, 100)))
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
	c.logger.Info("Converting DIB image data to PNG")
	imgData, err := c.convertDIBtoPNG(ptr, format)
	if err != nil {
		return nil, fmt.Errorf("failed to convert image: %v", err)
	}
	
	c.logger.Info("Converted image size", zap.Int("size", len(imgData)))
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
	c.logger.Info("File drop contains", zap.Int("count", count))
	
	// If there's just one file, treat it as FilePath
	if count == 1 {
		// Buffer for filename
		buf := make([]uint16, windows.MAX_PATH)
		windows.DragQueryFile(h, 0, &buf[0], windows.MAX_PATH)
		filePath := windows.UTF16ToString(buf)
		
		c.logger.Info("Single file path detected", zap.String("path", filePath))
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
		
		c.logger.Info("Multiple files detected", zap.Int("json_size", len(fileData)))
		return &types.ClipboardContent{
			Type:    types.TypeFile,
			Data:    fileData,
			Created: time.Now(),
		}, nil
	}
}

// readHTMLFormat reads HTML content from the clipboard (CF_HTML)
func (c *WindowsClipboard) readHTMLFormat() (*types.ClipboardContent, error) {
	h, err := windows.GetClipboardData(cfHTML)
	if err != nil {
		return nil, fmt.Errorf("failed to get HTML clipboard data: %v", err)
	}
	ptr, err := windows.GlobalLock(h)
	if err != nil {
		return nil, fmt.Errorf("failed to lock HTML clipboard memory: %v", err)
	}
	defer windows.GlobalUnlock(h)
	// Read the data as a byte slice
	// Find the length (null-terminated)
	var data []byte
	for i := 0; ; i++ {
		b := *(*byte)(unsafe.Pointer(uintptr(ptr) + uintptr(i)))
		if b == 0 {
			break
		}
		data = append(data, b)
	}
	html, err := parseCFHTML(data)
	if err != nil {
		c.logger.Warn("Failed to parse CF_HTML", zap.Error(err))
		return nil, err
	}
	c.logger.Info("Read HTML from clipboard", zap.Int("size", len(html)))
	return &types.ClipboardContent{
		Type:    types.TypeHTML,
		Data:    []byte(html),
		Created: time.Now(),
	}, nil
}

// parseCFHTML extracts the HTML fragment from CF_HTML clipboard data
func parseCFHTML(data []byte) (string, error) {
	s := string(data)
	startIdx := -1
	endIdx := -1
	// Look for StartHTML and EndHTML markers
	startMarker := "StartHTML:"
	endMarker := "EndHTML:"
	startPos := regexp.MustCompile(`StartHTML:(\\d{10})`).FindStringSubmatch(s)
	endPos := regexp.MustCompile(`EndHTML:(\\d{10})`).FindStringSubmatch(s)
	if len(startPos) > 1 {
		startIdx = atoiSafe(startPos[1])
	}
	if len(endPos) > 1 {
		endIdx = atoiSafe(endPos[1])
	}
	if startIdx >= 0 && endIdx > startIdx && endIdx <= len(s) {
		return s[startIdx:endIdx], nil
	}
	// Fallback: try to find <html>...</html>
	htmlStart := regexp.MustCompile(`(?i)<html[\s\S]*?>`).FindStringIndex(s)
	htmlEnd := regexp.MustCompile(`(?i)</html>`).FindStringIndex(s)
	if htmlStart != nil && htmlEnd != nil && htmlEnd[1] > htmlStart[0] {
		return s[htmlStart[0]:htmlEnd[1]], nil
	}
	return "", fmt.Errorf("could not extract HTML fragment from clipboard data")
}

func atoiSafe(s string) int {
	if len(s) == 0 {
		return -1
	}
	var n int
	fmt.Sscanf(s, "%d", &n)
	return n
}

// buildCFHTML constructs the HTML clipboard format for writing
func buildCFHTML(html string) []byte {
	// See: https://docs.microsoft.com/en-us/windows/win32/dataxchg/html-clipboard-format
	const headerTpl = "Version:0.9\r\nStartHTML:%010d\r\nEndHTML:%010d\r\nStartFragment:%010d\r\nEndFragment:%010d\r\n"
	fragmentStart := len(headerTpl) + 40 // 40 is the length of the header with all offsets
	fragmentEnd := fragmentStart + len(html)
	header := fmt.Sprintf(headerTpl, fragmentStart, fragmentEnd, fragmentStart, fragmentEnd)
	return append([]byte(header), []byte(html)...)
}

// readRTFFormat reads RTF content from the clipboard (CF_RTF)
func (c *WindowsClipboard) readRTFFormat() (*types.ClipboardContent, error) {
	h, err := windows.GetClipboardData(cfRTF)
	if err != nil {
		return nil, fmt.Errorf("failed to get RTF clipboard data: %v", err)
	}
	ptr, err := windows.GlobalLock(h)
	if err != nil {
		return nil, fmt.Errorf("failed to lock RTF clipboard memory: %v", err)
	}
	defer windows.GlobalUnlock(h)
	// Read the data as a byte slice (null-terminated)
	var data []byte
	for i := 0; ; i++ {
		b := *(*byte)(unsafe.Pointer(uintptr(ptr) + uintptr(i)))
		if b == 0 {
			break
		}
		data = append(data, b)
	}
	c.logger.Info("Read RTF from clipboard", zap.Int("size", len(data)))
	return &types.ClipboardContent{
		Type:    types.TypeRTF,
		Data:    data,
		Created: time.Now(),
	}, nil
}

// MonitorChanges monitors for clipboard changes and sends updates to the channel
func (c *WindowsClipboard) MonitorChanges(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
	c.logger.Info("Starting clipboard monitoring")
	
	go func() {
		for {
			select {
			case <-stopCh:
				c.logger.Info("Received stop signal, ending clipboard monitoring")
				return
				
			case <-c.msgChan:
				// Clipboard changed, read the new content
				c.logger.Info("Processing clipboard change notification")
				content, err := c.Read()
				if err != nil {
					if err.Error() == "content unchanged" {
						c.logger.Info("Content unchanged, skipping")
						continue
					}
					c.logger.Error("Error reading clipboard", zap.Error(err))
					continue
				}
				
				// Send the content to the channel
				select {
				case contentCh <- content:
					c.logger.Info("Sent new clipboard content to channel", zap.String("type", content.Type), zap.Int("size", len(content.Data)))
				case <-stopCh:
					c.logger.Info("Received stop signal while sending content")
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
				c.logger.Info("Clipboard window thread exited, restarting")
				
				c.mu.Lock()
				isRunning := c.isRunning
				c.mu.Unlock()
				
				if !isRunning {
					c.windowThread = make(chan struct{})
					go c.createClipboardWindow()
					c.logger.Info("Clipboard window restarted")
				}
			}
		}
	}()
}

// Write sets the clipboard content
func (c *WindowsClipboard) Write(content *types.ClipboardContent) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.logger.Info("Writing to clipboard", zap.String("type", content.Type), zap.Int("size", len(content.Data)), zap.Uintptr("handle", uintptr(content.Handle)))
	
	switch content.Type {
	case types.TypeText, types.TypeString, types.TypeURL:
		return c.writeTextFormat(content)
	case types.TypeImage:
		return c.writeImageFormat(content)
	case types.TypeHTML:
		return c.writeHTMLFormat(content)
	case types.TypeRTF:
		return c.writeRTFFormat(content)
	case types.TypeBitmapHandle:
		return c.writeBitmapHandle(content)
	case types.TypeMetafileHandle:
		return c.writeEnhMetafileHandle(content)
	case types.TypeFilePath, types.TypeFile:
		return fmt.Errorf("writing files to clipboard is not implemented yet")
	default:
		return fmt.Errorf("unsupported content type: %s", content.Type)
	}
}

// writeTextFormat writes text content to clipboard
func (c *WindowsClipboard) writeTextFormat(content *types.ClipboardContent) error {
	c.logger.Info("Writing text to clipboard", zap.String("text", truncateString(string(content.Data), 100)))
	
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
	
	c.logger.Info("Successfully wrote text to clipboard")
	return nil
}

// writeImageFormat writes image content to clipboard
func (c *WindowsClipboard) writeImageFormat(content *types.ClipboardContent) error {
	c.logger.Info("Writing image to clipboard", zap.Int("size", len(content.Data)))
	if len(content.Data) == 0 {
		return fmt.Errorf("no image data to write")
	}
	// Decode image (support PNG, JPEG, etc.)
	img, _, err := image.Decode(bytes.NewReader(content.Data))
	if err != nil {
		c.logger.Error("Failed to decode image", zap.Error(err))
		return fmt.Errorf("failed to decode image: %w", err)
	}
	// Convert to DIB format (BGRA, bottom-up)
	dib, err := encodeToDIB(img)
	if err != nil {
		c.logger.Error("Failed to encode image to DIB", zap.Error(err))
		return fmt.Errorf("failed to encode image to DIB: %w", err)
	}
	// Open clipboard
	err = windows.OpenClipboard(0)
	if err != nil {
		return fmt.Errorf("failed to open clipboard: %v", err)
	}
	defer windows.CloseClipboard()
	// Empty clipboard
	err = windows.EmptyClipboard()
	if err != nil {
		return fmt.Errorf("failed to empty clipboard: %v", err)
	}
	// Allocate global memory for DIB
	h, err := windows.GlobalAlloc(windows.GMEM_MOVEABLE, uint32(len(dib)))
	if err != nil {
		return fmt.Errorf("failed to allocate memory: %v", err)
	}
	ptr, err := windows.GlobalLock(h)
	if err != nil {
		windows.GlobalFree(h)
		return fmt.Errorf("failed to lock memory: %v", err)
	}
	copy((*[1 << 30]byte)(unsafe.Pointer(ptr))[:len(dib)], dib)
	windows.GlobalUnlock(h)
	// Set clipboard data as CF_DIB
	if _, err := windows.SetClipboardData(windows.CF_DIB, h); err != nil {
		windows.GlobalFree(h)
		return fmt.Errorf("failed to set clipboard data: %v", err)
	}
	c.logger.Info("Successfully wrote image to clipboard as DIB", zap.Int("size", len(dib)))
	return nil
}

// writeHTMLFormat writes HTML content to the clipboard (CF_HTML)
func (c *WindowsClipboard) writeHTMLFormat(content *types.ClipboardContent) error {
	c.logger.Info("Writing HTML to clipboard", zap.Int("size", len(content.Data)))
	if len(content.Data) == 0 {
		return fmt.Errorf("no HTML data to write")
	}
	html := string(content.Data)
	cfhtml := buildCFHTML(html)
	// Open clipboard
	err := windows.OpenClipboard(0)
	if err != nil {
		return fmt.Errorf("failed to open clipboard: %v", err)
	}
	defer windows.CloseClipboard()
	// Empty clipboard
	err = windows.EmptyClipboard()
	if err != nil {
		return fmt.Errorf("failed to empty clipboard: %v", err)
	}
	// Allocate global memory for HTML
	h, err := windows.GlobalAlloc(windows.GMEM_MOVEABLE, uint32(len(cfhtml)))
	if err != nil {
		return fmt.Errorf("failed to allocate memory: %v", err)
	}
	ptr, err := windows.GlobalLock(h)
	if err != nil {
		windows.GlobalFree(h)
		return fmt.Errorf("failed to lock memory: %v", err)
	}
	copy((*[1 << 30]byte)(unsafe.Pointer(ptr))[:len(cfhtml)], cfhtml)
	windows.GlobalUnlock(h)
	// Set clipboard data as CF_HTML
	if _, err := windows.SetClipboardData(cfHTML, h); err != nil {
		windows.GlobalFree(h)
		return fmt.Errorf("failed to set clipboard data: %v", err)
	}
	c.logger.Info("Successfully wrote HTML to clipboard as CF_HTML", zap.Int("size", len(cfhtml)))
	return nil
}

// writeRTFFormat writes RTF content to the clipboard (CF_RTF)
func (c *WindowsClipboard) writeRTFFormat(content *types.ClipboardContent) error {
	c.logger.Info("Writing RTF to clipboard", zap.Int("size", len(content.Data)))
	if len(content.Data) == 0 {
		return fmt.Errorf("no RTF data to write")
	}
	// Open clipboard
	err := windows.OpenClipboard(0)
	if err != nil {
		return fmt.Errorf("failed to open clipboard: %v", err)
	}
	defer windows.CloseClipboard()
	// Empty clipboard
	err = windows.EmptyClipboard()
	if err != nil {
		return fmt.Errorf("failed to empty clipboard: %v", err)
	}
	// Allocate global memory for RTF
	h, err := windows.GlobalAlloc(windows.GMEM_MOVEABLE, uint32(len(content.Data)))
	if err != nil {
		return fmt.Errorf("failed to allocate memory: %v", err)
	}
	ptr, err := windows.GlobalLock(h)
	if err != nil {
		windows.GlobalFree(h)
		return fmt.Errorf("failed to lock memory: %v", err)
	}
	copy((*[1 << 30]byte)(unsafe.Pointer(ptr))[:len(content.Data)], content.Data)
	windows.GlobalUnlock(h)
	// Set clipboard data as CF_RTF
	if _, err := windows.SetClipboardData(cfRTF, h); err != nil {
		windows.GlobalFree(h)
		return fmt.Errorf("failed to set RTF clipboard data: %v", err)
	}
	c.logger.Info("Successfully wrote RTF to clipboard", zap.Int("size", len(content.Data)))
	return nil
}

// encodeToDIB encodes a Go image.Image to DIB (BGRA, bottom-up, uncompressed)
func encodeToDIB(img image.Image) ([]byte, error) {
	// Only support RGBA images for now
	b := img.Bounds()
	width := b.Dx()
	height := b.Dy()
	bitCount := 32 // BGRA
	rowSize := ((width*bitCount + 31) / 32) * 4
	imgSize := rowSize * height
	headerSize := int(unsafe.Sizeof(windows.BITMAPINFOHEADER{}))
	dib := make([]byte, headerSize+imgSize)
	// Fill BITMAPINFOHEADER
	header := (*windows.BITMAPINFOHEADER)(unsafe.Pointer(&dib[0]))
	header.BiSize = uint32(headerSize)
	header.BiWidth = int32(width)
	header.BiHeight = int32(height) // bottom-up
	header.BiPlanes = 1
	header.BiBitCount = uint16(bitCount)
	header.BiCompression = 0 // BI_RGB
	header.BiSizeImage = uint32(imgSize)
	header.BiXPelsPerMeter = 0
	header.BiYPelsPerMeter = 0
	header.BiClrUsed = 0
	header.BiClrImportant = 0
	// Write pixel data (bottom-up, BGRA)
	offset := headerSize
	for y := height - 1; y >= 0; y-- {
		for x := 0; x < width; x++ {
			r, g, b, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			dib[offset+0] = byte(b >> 8)
			dib[offset+1] = byte(g >> 8)
			dib[offset+2] = byte(r >> 8)
			dib[offset+3] = byte(a >> 8)
			offset += 4
		}
	}
	return dib, nil
}

// Close releases any resources
func (c *WindowsClipboard) Close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.logger.Info("Closing clipboard resources")
	
	if c.hwnd != 0 {
		c.logger.Info("Removing clipboard format listener and destroying window")
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

// readBitmapHandleFormat handles CF_BITMAP (HBITMAP) clipboard data
// Populates ClipboardContent.Handle for local use only. Never send this handle across processes or platforms.
func (c *WindowsClipboard) readBitmapHandleFormat() (*types.ClipboardContent, error) {
	h, err := windows.GetClipboardData(windows.CF_BITMAP)
	if err != nil {
		c.logger.Warn("Failed to get HBITMAP from clipboard", zap.Error(err))
		return nil, fmt.Errorf("failed to get HBITMAP from clipboard: %w", err)
	}
	c.logger.Info("Read HBITMAP handle from clipboard (local use only)", zap.Uintptr("handle", uintptr(h)))
	return &types.ClipboardContent{
		Type:   types.TypeBitmapHandle,
		Handle: h,
		Data:   nil,
		Created: time.Now(),
	}, nil
}

// readEnhMetafileFormat handles CF_ENHMETAFILE (HENHMETAFILE) clipboard data
// Populates ClipboardContent.Handle for local use only. Never send this handle across processes or platforms.
func (c *WindowsClipboard) readEnhMetafileFormat() (*types.ClipboardContent, error) {
	h, err := windows.GetClipboardData(windows.CF_ENHMETAFILE)
	if err != nil {
		c.logger.Warn("Failed to get HENHMETAFILE from clipboard", zap.Error(err))
		return nil, fmt.Errorf("failed to get HENHMETAFILE from clipboard: %w", err)
	}
	c.logger.Info("Read HENHMETAFILE handle from clipboard (local use only)", zap.Uintptr("handle", uintptr(h)))
	return &types.ClipboardContent{
		Type:   types.TypeMetafileHandle,
		Handle: h,
		Data:   nil,
		Created: time.Now(),
	}, nil
}

// writeBitmapHandle writes a HBITMAP handle to the clipboard (local use only)
func (c *WindowsClipboard) writeBitmapHandle(content *types.ClipboardContent) error {
	if content.Handle == 0 {
		return fmt.Errorf("no HBITMAP handle provided")
	}
	c.logger.Info("Writing HBITMAP handle to clipboard (local use only)", zap.Uintptr("handle", uintptr(content.Handle)))
	if err := windows.OpenClipboard(0); err != nil {
		return fmt.Errorf("failed to open clipboard: %v", err)
	}
	defer windows.CloseClipboard()
	if err := windows.EmptyClipboard(); err != nil {
		return fmt.Errorf("failed to empty clipboard: %v", err)
	}
	if _, err := windows.SetClipboardData(windows.CF_BITMAP, content.Handle); err != nil {
		return fmt.Errorf("failed to set HBITMAP to clipboard: %v", err)
	}
	return nil
}

// writeEnhMetafileHandle writes a HENHMETAFILE handle to the clipboard (local use only)
func (c *WindowsClipboard) writeEnhMetafileHandle(content *types.ClipboardContent) error {
	if content.Handle == 0 {
		return fmt.Errorf("no HENHMETAFILE handle provided")
	}
	c.logger.Info("Writing HENHMETAFILE handle to clipboard (local use only)", zap.Uintptr("handle", uintptr(content.Handle)))
	if err := windows.OpenClipboard(0); err != nil {
		return fmt.Errorf("failed to open clipboard: %v", err)
	}
	defer windows.CloseClipboard()
	if err := windows.EmptyClipboard(); err != nil {
		return fmt.Errorf("failed to empty clipboard: %v", err)
	}
	if _, err := windows.SetClipboardData(windows.CF_ENHMETAFILE, content.Handle); err != nil {
		return fmt.Errorf("failed to set HENHMETAFILE to clipboard: %v", err)
	}
	return nil
}

// Conversion helpers for cross-platform/GUI use
// These must be called by higher-level code when you need to send or display the content.

// ConvertBitmapHandleToPNG converts a HBITMAP handle to PNG bytes (for cross-platform use)
func ConvertBitmapHandleToPNG(h windows.Handle) ([]byte, error) {
	// TODO: Implement HBITMAP -> DIB -> PNG conversion using GDI APIs
	return nil, fmt.Errorf("HBITMAP to PNG conversion not implemented")
}

// ConvertEnhMetafileToPNG converts a HENHMETAFILE handle to PNG bytes (for cross-platform use)
func ConvertEnhMetafileToPNG(h windows.Handle) ([]byte, error) {
	// TODO: Implement HENHMETAFILE -> PNG conversion using GDI APIs
	return nil, fmt.Errorf("HENHMETAFILE to PNG conversion not implemented")
}

// ConvertEnhMetafileToSVG converts a HENHMETAFILE handle to SVG bytes (for cross-platform use)
func ConvertEnhMetafileToSVG(h windows.Handle) ([]byte, error) {
	// TODO: Implement HENHMETAFILE -> SVG conversion (advanced, may require third-party libs)
	return nil, fmt.Errorf("HENHMETAFILE to SVG conversion not implemented")
}