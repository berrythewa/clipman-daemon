//go:build linux
// +build linux

package platform

/*
#cgo pkg-config: x11 xfixes
#cgo CFLAGS: -I/usr/include/X11
#include <X11/Xlib.h>
#include <X11/Xatom.h>
#include <X11/extensions/Xfixes.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <sys/select.h>
#include <sys/time.h>
#include <errno.h>

typedef struct {
    Display *display;
    Window window;
    Atom clipboard;
    Atom primary;
    Atom targets;
    Atom text;
    Atom utf8_string;
    Atom html;
    Atom rtf;
    Atom uri_list;
    Atom image_png;
    Atom image_jpeg;
    Atom image_bmp;
    Atom image_gif;
    Atom gnome_copied_files;
    int xfixes_event_base;
    int xfixes_error_base;
    int has_xfixes;
} DirectClipboard;

DirectClipboard* init_direct_clipboard() {
    DirectClipboard *dc = malloc(sizeof(DirectClipboard));
    if (!dc) return NULL;
    
    memset(dc, 0, sizeof(DirectClipboard));
    
    dc->display = XOpenDisplay(NULL);
    if (!dc->display) {
        free(dc);
        return NULL;
    }
    
    // Create a minimal invisible window for selection requests
    dc->window = XCreateSimpleWindow(dc->display, DefaultRootWindow(dc->display),
                                    0, 0, 1, 1, 0, 0, 0);
    
    // Intern atoms
    dc->clipboard = XInternAtom(dc->display, "CLIPBOARD", False);
    dc->primary = XInternAtom(dc->display, "PRIMARY", False);
    dc->targets = XInternAtom(dc->display, "TARGETS", False);
    dc->text = XInternAtom(dc->display, "TEXT", False);
    dc->utf8_string = XInternAtom(dc->display, "UTF8_STRING", False);
    dc->html = XInternAtom(dc->display, "text/html", False);
    dc->rtf = XInternAtom(dc->display, "text/rtf", False);
    dc->uri_list = XInternAtom(dc->display, "text/uri-list", False);
    dc->image_png = XInternAtom(dc->display, "image/png", False);
    dc->image_jpeg = XInternAtom(dc->display, "image/jpeg", False);
    dc->image_bmp = XInternAtom(dc->display, "image/bmp", False);
    dc->image_gif = XInternAtom(dc->display, "image/gif", False);
    dc->gnome_copied_files = XInternAtom(dc->display, "x-special/gnome-copied-files", False);
    
    // Check for XFixes extension
    dc->has_xfixes = XFixesQueryExtension(dc->display, &dc->xfixes_event_base, &dc->xfixes_error_base);
    
    return dc;
}

void cleanup_direct_clipboard(DirectClipboard *dc) {
    if (dc) {
        if (dc->window) {
            XDestroyWindow(dc->display, dc->window);
        }
        if (dc->display) {
            XCloseDisplay(dc->display);
        }
        free(dc);
    }
}

// Get available targets (formats) from clipboard
char** get_clipboard_targets(DirectClipboard *dc, int *count) {
    if (!dc || !dc->display) {
        *count = 0;
        return NULL;
    }
    
    // Request targets
    XConvertSelection(dc->display, dc->clipboard, dc->targets,
                     dc->targets, dc->window, CurrentTime);
    XFlush(dc->display);
    
    // Wait for selection notify
    XEvent event;
    fd_set fds;
    struct timeval timeout;
    
    FD_ZERO(&fds);
    FD_SET(ConnectionNumber(dc->display), &fds);
    timeout.tv_sec = 1;
    timeout.tv_usec = 0;
    
    while (select(ConnectionNumber(dc->display) + 1, &fds, NULL, NULL, &timeout) > 0) {
        XNextEvent(dc->display, &event);
        
        if (event.type == SelectionNotify) {
            if (event.xselection.property != None) {
                // Get the property
                Atom actual_type;
                int actual_format;
                unsigned long nitems, bytes_after;
                unsigned char *data;
                
                XGetWindowProperty(dc->display, dc->window, dc->targets,
                                  0, 65536, False, AnyPropertyType,
                                  &actual_type, &actual_format, &nitems,
                                  &bytes_after, &data);
                
                if (data && actual_format == 32) {
                    Atom *atoms = (Atom*)data;
                    char **targets = malloc(nitems * sizeof(char*));
                    if (!targets) {
                        XFree(data);
                        *count = 0;
                        return NULL;
                    }
                    
                    for (unsigned long i = 0; i < nitems; i++) {
                        char *name = XGetAtomName(dc->display, atoms[i]);
                        if (name) {
                            targets[i] = strdup(name);
                            XFree(name);
                        } else {
                            targets[i] = NULL;
                        }
                    }
                    
                    XFree(data);
                    *count = nitems;
                    return targets;
                }
                if (data) XFree(data);
            }
            break;
        }
    }
    
    *count = 0;
    return NULL;
}

// Read clipboard content with specific target
char* read_clipboard_target(DirectClipboard *dc, Atom target) {
    if (!dc || !dc->display) return NULL;
    
    // Request clipboard content
    XConvertSelection(dc->display, dc->clipboard, target,
                     target, dc->window, CurrentTime);
    XFlush(dc->display);
    
    // Wait for selection notify
    XEvent event;
    fd_set fds;
    struct timeval timeout;
    
    FD_ZERO(&fds);
    FD_SET(ConnectionNumber(dc->display), &fds);
    timeout.tv_sec = 1;
    timeout.tv_usec = 0;
    
    while (select(ConnectionNumber(dc->display) + 1, &fds, NULL, NULL, &timeout) > 0) {
        XNextEvent(dc->display, &event);
        
        if (event.type == SelectionNotify) {
            if (event.xselection.property != None) {
                // Get the property
                Atom actual_type;
                int actual_format;
                unsigned long nitems, bytes_after;
                unsigned char *data;
                
                XGetWindowProperty(dc->display, dc->window, target,
                                  0, 65536, False, AnyPropertyType,
                                  &actual_type, &actual_format, &nitems,
                                  &bytes_after, &data);
                
                if (data) {
                    char *result = malloc(nitems + 1);
                    if (result) {
                        memcpy(result, data, nitems);
                        result[nitems] = '\0';
                    }
                    XFree(data);
                    return result;
                }
            }
            break;
        }
    }
    
    return NULL;
}

// Write text to clipboard
int write_clipboard_text(DirectClipboard *dc, const char *text) {
    if (!dc || !dc->display || !text) return -1;
    
    // Set the selection owner
    XSetSelectionOwner(dc->display, dc->clipboard, dc->window, CurrentTime);
    XFlush(dc->display);
    
    // Check if we got ownership
    if (XGetSelectionOwner(dc->display, dc->clipboard) != dc->window) {
        return -1;
    }
    
    return 0;
}

// Check if XFixes extension is available
int has_xfixes_support(DirectClipboard *dc) {
    return dc ? dc->has_xfixes : 0;
}

// Setup XFixes monitoring
int setup_xfixes_monitoring(DirectClipboard *dc) {
    if (!dc || !dc->has_xfixes) return -1;
    
    XFixesSelectSelectionInput(dc->display, dc->window, dc->clipboard,
                              XFixesSetSelectionOwnerNotifyMask);
    return dc->xfixes_event_base;
}

// Check for XFixes events
int check_xfixes_event(DirectClipboard *dc) {
    if (!dc || !dc->has_xfixes) return 0;
    
    XEvent event;
    if (XCheckTypedEvent(dc->display, dc->xfixes_event_base + XFixesSelectionNotify, &event)) {
        XFixesSelectionNotifyEvent *notify = (XFixesSelectionNotifyEvent*)&event;
        if (notify->selection == dc->clipboard) {
            return 1;
        }
    }
    
    return 0;
}

// Get display name
char* get_display_name() {
    const char *display = getenv("DISPLAY");
    if (display) {
        return strdup(display);
    }
    return strdup(":0");
}

// Check if display is available
int is_display_available() {
    Display *display = XOpenDisplay(NULL);
    if (display) {
        XCloseDisplay(display);
        return 1;
    }
    return 0;
}
*/
import "C"
import (
	"fmt"
	"strings"
	"time"
	"unsafe"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"go.uber.org/zap"
)

// DirectClipboard provides direct X11 clipboard access without external tools
type DirectClipboard struct {
	dc *C.DirectClipboard
}

// NewDirectClipboard creates a new direct clipboard instance
func NewDirectClipboard() (*DirectClipboard, error) {
	// Check if X11 display is available
	if C.is_display_available() == 0 {
		return nil, fmt.Errorf("X11 display not available")
	}
	
	dc := C.init_direct_clipboard()
	if dc == nil {
		return nil, fmt.Errorf("failed to initialize direct clipboard")
	}
	
	return &DirectClipboard{dc: dc}, nil
}

// Close cleans up the direct clipboard instance
func (d *DirectClipboard) Close() {
	if d.dc != nil {
		C.cleanup_direct_clipboard(d.dc)
		d.dc = nil
	}
}

// GetTargets returns available clipboard formats
func (d *DirectClipboard) GetTargets() ([]string, error) {
	if d.dc == nil {
		return nil, fmt.Errorf("direct clipboard not initialized")
	}
	
	var count C.int
	targets := C.get_clipboard_targets(d.dc, &count)
	if targets == nil {
		return []string{}, nil
	}
	
	defer C.free(unsafe.Pointer(targets))
	
	result := make([]string, count)
	for i := 0; i < int(count); i++ {
		targetPtr := (*[1 << 30]*C.char)(unsafe.Pointer(targets))[i]
		if targetPtr != nil {
			result[i] = C.GoString(targetPtr)
			C.free(unsafe.Pointer(targetPtr))
		}
	}
	
	return result, nil
}

// ReadText reads text content from clipboard
func (d *DirectClipboard) ReadText() (string, error) {
	if d.dc == nil {
		return "", fmt.Errorf("direct clipboard not initialized")
	}
	
	// Try UTF8_STRING first
	cstr := C.read_clipboard_target(d.dc, d.dc.utf8_string)
	if cstr != nil {
		defer C.free(unsafe.Pointer(cstr))
		return C.GoString(cstr), nil
	}
	
	// Fallback to TEXT
	cstr = C.read_clipboard_target(d.dc, d.dc.text)
	if cstr != nil {
		defer C.free(unsafe.Pointer(cstr))
		return C.GoString(cstr), nil
	}
	
	return "", nil // Empty clipboard
}

// ReadHTML reads HTML content from clipboard
func (d *DirectClipboard) ReadHTML() (string, error) {
	if d.dc == nil {
		return "", fmt.Errorf("direct clipboard not initialized")
	}
	
	cstr := C.read_clipboard_target(d.dc, d.dc.html)
	if cstr != nil {
		defer C.free(unsafe.Pointer(cstr))
		return C.GoString(cstr), nil
	}
	
	return "", nil
}

// ReadRTF reads RTF content from clipboard
func (d *DirectClipboard) ReadRTF() (string, error) {
	if d.dc == nil {
		return "", fmt.Errorf("direct clipboard not initialized")
	}
	
	cstr := C.read_clipboard_target(d.dc, d.dc.rtf)
	if cstr != nil {
		defer C.free(unsafe.Pointer(cstr))
		return C.GoString(cstr), nil
	}
	
	return "", nil
}

// ReadFiles reads file list from clipboard
func (d *DirectClipboard) ReadFiles() (string, error) {
	if d.dc == nil {
		return "", fmt.Errorf("direct clipboard not initialized")
	}
	
	// Try URI list first
	cstr := C.read_clipboard_target(d.dc, d.dc.uri_list)
	if cstr != nil {
		defer C.free(unsafe.Pointer(cstr))
		return C.GoString(cstr), nil
	}
	
	// Try GNOME copied files
	cstr = C.read_clipboard_target(d.dc, d.dc.gnome_copied_files)
	if cstr != nil {
		defer C.free(unsafe.Pointer(cstr))
		return C.GoString(cstr), nil
	}
	
	return "", nil
}

// ReadImage reads image data from clipboard
func (d *DirectClipboard) ReadImage() ([]byte, error) {
	if d.dc == nil {
		return nil, fmt.Errorf("direct clipboard not initialized")
	}
	
	// Try different image formats
	imageTargets := []C.Atom{
		d.dc.image_png,
		d.dc.image_jpeg,
		d.dc.image_bmp,
		d.dc.image_gif,
	}
	
	for _, target := range imageTargets {
		cstr := C.read_clipboard_target(d.dc, target)
		if cstr != nil {
			defer C.free(unsafe.Pointer(cstr))
			// Convert to byte slice
			length := C.strlen(cstr)
			data := make([]byte, length)
			copy(data, C.GoBytes(unsafe.Pointer(cstr), C.int(length)))
			return data, nil
		}
	}
	
	return nil, nil
}

// WriteText writes text to clipboard
func (d *DirectClipboard) WriteText(text string) error {
	if d.dc == nil {
		return fmt.Errorf("direct clipboard not initialized")
	}
	
	cstr := C.CString(text)
	defer C.free(unsafe.Pointer(cstr))
	
	if C.write_clipboard_text(d.dc, cstr) != 0 {
		return fmt.Errorf("failed to write text to clipboard")
	}
	
	return nil
}

// HasXFixesSupport checks if XFixes extension is available
func (d *DirectClipboard) HasXFixesSupport() bool {
	if d.dc == nil {
		return false
	}
	return C.has_xfixes_support(d.dc) != 0
}

// MonitorChanges monitors clipboard changes using XFixes extension
func (d *DirectClipboard) MonitorChanges(callback func(string)) error {
	if d.dc == nil {
		return fmt.Errorf("direct clipboard not initialized")
	}
	
	// Setup XFixes monitoring
	eventBase := C.setup_xfixes_monitoring(d.dc)
	if eventBase < 0 {
		return fmt.Errorf("XFixes extension not available")
	}
	
	go func() {
		for {
			// Check for XFixes events
			if C.check_xfixes_event(d.dc) != 0 {
				content, err := d.ReadText()
				if err == nil && content != "" {
					callback(content)
				}
			}
			time.Sleep(100 * time.Millisecond)
		}
	}()
	
	return nil
}

// DirectClipboardBackend implements clipboard access using direct X11 calls
type DirectClipboardBackend struct {
	dc     *DirectClipboard
	logger *zap.Logger
}

// NewDirectClipboardBackend creates a new direct clipboard backend
func NewDirectClipboardBackend(logger *zap.Logger) (*DirectClipboardBackend, error) {
	dc, err := NewDirectClipboard()
	if err != nil {
		return nil, err
	}
	
	return &DirectClipboardBackend{
		dc:     dc,
		logger: logger,
	}, nil
}

// Read implements clipboard reading with format detection
func (d *DirectClipboardBackend) Read() (*types.ClipboardContent, error) {
	if d.dc == nil {
		return nil, fmt.Errorf("direct clipboard not initialized")
	}
	
	// Get available targets to determine content type
	targets, err := d.dc.GetTargets()
	if err != nil {
		d.logger.Debug("Failed to get clipboard targets", zap.Error(err))
	}
	
	d.logger.Debug("Available clipboard targets", zap.Strings("targets", targets))
	
	// Try to read different content types in priority order
	
	// 1. Try HTML
	if html, err := d.dc.ReadHTML(); err == nil && html != "" {
		d.logger.Debug("Read HTML content from clipboard", zap.Int("length", len(html)))
		return &types.ClipboardContent{
			Type: types.TypeHTML,
			Data: []byte(html),
		}, nil
	}
	
	// 2. Try RTF
	if rtf, err := d.dc.ReadRTF(); err == nil && rtf != "" {
		d.logger.Debug("Read RTF content from clipboard", zap.Int("length", len(rtf)))
		return &types.ClipboardContent{
			Type: types.TypeRTF,
			Data: []byte(rtf),
		}, nil
	}
	
	// 3. Try files
	if files, err := d.dc.ReadFiles(); err == nil && files != "" {
		d.logger.Debug("Read file list from clipboard", zap.String("files", files))
		return &types.ClipboardContent{
			Type: types.TypeFile,
			Data: []byte(files),
		}, nil
	}
	
	// 4. Try image
	if image, err := d.dc.ReadImage(); err == nil && len(image) > 0 {
		d.logger.Debug("Read image from clipboard", zap.Int("size", len(image)))
		return &types.ClipboardContent{
			Type: types.TypeImage,
			Data: image,
		}, nil
	}
	
	// 5. Try text (fallback)
	if text, err := d.dc.ReadText(); err == nil && text != "" {
		d.logger.Debug("Read text from clipboard", zap.Int("length", len(text)))
		
		// Detect if it's a URL
		if strings.HasPrefix(text, "http://") || strings.HasPrefix(text, "https://") {
			return &types.ClipboardContent{
				Type: types.TypeURL,
				Data: []byte(text),
			}, nil
		}
		
		return &types.ClipboardContent{
			Type: types.TypeText,
			Data: []byte(text),
		}, nil
	}
	
	return nil, fmt.Errorf("no content available in clipboard")
}

// Write implements clipboard writing
func (d *DirectClipboardBackend) Write(content *types.ClipboardContent) error {
	if d.dc == nil {
		return fmt.Errorf("direct clipboard not initialized")
	}
	
	if content == nil || len(content.Data) == 0 {
		return fmt.Errorf("empty content")
	}
	
	switch content.Type {
	case types.TypeText, types.TypeString, types.TypeURL:
		return d.dc.WriteText(string(content.Data))
	default:
		return fmt.Errorf("unsupported content type for direct clipboard: %s", content.Type)
	}
}

// MonitorChanges implements clipboard monitoring
func (d *DirectClipboardBackend) MonitorChanges(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
	d.logger.Info("Starting direct clipboard monitoring")
	
	// Use XFixes if available, otherwise fall back to polling
	if d.dc.HasXFixesSupport() {
		d.monitorWithXFixes(contentCh, stopCh)
	} else {
		d.monitorWithPolling(contentCh, stopCh)
	}
}

// monitorWithXFixes monitors using XFixes extension
func (d *DirectClipboardBackend) monitorWithXFixes(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
	d.logger.Info("Using XFixes for clipboard monitoring")
	
	// Setup XFixes monitoring
	eventBase := C.setup_xfixes_monitoring(d.dc.dc)
	if eventBase < 0 {
		d.logger.Warn("Failed to setup XFixes monitoring, falling back to polling")
		d.monitorWithPolling(contentCh, stopCh)
		return
	}
	
	go func() {
		for {
			select {
			case <-stopCh:
				d.logger.Info("Stopping XFixes clipboard monitoring")
				return
			default:
				// Check for XFixes events
				if C.check_xfixes_event(d.dc.dc) != 0 {
					content, err := d.Read()
					if err == nil {
						select {
						case contentCh <- content:
							d.logger.Debug("Sent clipboard content via XFixes")
						case <-stopCh:
							return
						}
					}
				}
				time.Sleep(50 * time.Millisecond)
			}
		}
	}()
}

// monitorWithPolling monitors using polling
func (d *DirectClipboardBackend) monitorWithPolling(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) {
	d.logger.Info("Using polling for clipboard monitoring")
	
	var lastContent string
	
	go func() {
		ticker := time.NewTicker(200 * time.Millisecond)
		defer ticker.Stop()
		
		for {
			select {
			case <-stopCh:
				d.logger.Info("Stopping polling clipboard monitoring")
				return
			case <-ticker.C:
				content, err := d.Read()
				if err == nil {
					currentContent := string(content.Data)
					if currentContent != lastContent {
						lastContent = currentContent
						select {
						case contentCh <- content:
							d.logger.Debug("Sent clipboard content via polling")
						case <-stopCh:
							return
						}
					}
				}
			}
		}
	}()
}

// Close implements cleanup
func (d *DirectClipboardBackend) Close() {
	if d.dc != nil {
		d.dc.Close()
		d.dc = nil
	}
}

// GetMonitoringStatus returns monitoring status
func (d *DirectClipboardBackend) GetMonitoringStatus() types.MonitoringStatus {
	mode := "polling"
	if d.dc != nil && d.dc.HasXFixesSupport() {
		mode = "xfixes"
	}
	
	return types.MonitoringStatus{
		IsRunning:    d.dc != nil,
		Mode:         mode,
		LastActivity: time.Now(),
		ErrorCount:   0,
		LastError:    "",
	}
}

// RestartMonitoring restarts clipboard monitoring
func (d *DirectClipboardBackend) RestartMonitoring(contentCh chan<- *types.ClipboardContent, stopCh <-chan struct{}) error {
	d.logger.Info("Restarting direct clipboard monitoring")
	
	// Close existing monitoring
	if d.dc != nil {
		d.dc.Close()
	}
	
	// Reinitialize
	dc, err := NewDirectClipboard()
	if err != nil {
		return fmt.Errorf("failed to reinitialize direct clipboard: %w", err)
	}
	
	d.dc = dc
	
	// Restart monitoring
	d.MonitorChanges(contentCh, stopCh)
	
	return nil
} 