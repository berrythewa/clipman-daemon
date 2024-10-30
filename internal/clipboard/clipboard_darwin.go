//go:build darwin

package clipboard

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>
#import <stdlib.h>
#import <string.h>

static int getChangeCount() {
    return [[NSPasteboard generalPasteboard] changeCount];
}

static void* getClipboardData(char** type) {
    NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
    NSArray *types = [pasteboard types];

    // Handle web content/URLs
    if ([types containsObject:NSPasteboardTypeURL]) {
        *type = strdup("url");
        NSURL *url = [NSURL URLFromPasteboard:pasteboard];
        return strdup([[url absoluteString] UTF8String]);
    }

    // Handle images from web
    if ([types containsObject:NSPasteboardTypeTIFF] ||
        [types containsObject:NSPasteboardTypePNG]) {
        *type = strdup("image");
        NSData *imageData;
        if ([types containsObject:NSPasteboardTypePNG]) {
            imageData = [pasteboard dataForType:NSPasteboardTypePNG];
        } else {
            imageData = [pasteboard dataForType:NSPasteboardTypeTIFF];
        }
        if (imageData) {
            void *bytes = malloc([imageData length]);
            memcpy(bytes, [imageData bytes], [imageData length]);
            return bytes;
        }
    }

    // Fall back to text
    if ([types containsObject:NSPasteboardTypeString]) {
        *type = strdup("text");
        NSString *text = [pasteboard stringForType:NSPasteboardTypeString];
        return strdup([text UTF8String]);
    }

    return NULL;
}
*/
import "C"
import (
	"github.com/berrythewa/clipman-daemon/internal/types"
	"time"
	"unsafe"
)

type DarwinClipboard struct {
	lastChangeCount C.int
}

func NewDarwinClipboard() *DarwinClipboard {
	return &DarwinClipboard{
		lastChangeCount: C.getChangeCount(),
	}
}

func (c *DarwinClipboard) Read() (*types.ClipboardContent, error) {
	var cType *C.char
	cData := C.getClipboardData(&cType)
	if cData == nil {
		return nil, nil
	}
	defer C.free(unsafe.Pointer(cData))

	contentType := C.GoString(cType)
	C.free(unsafe.Pointer(cType))

	content := &types.ClipboardContent{
		Data:    C.GoBytes(unsafe.Pointer(cData), C.int(C.strlen((*C.char)(cData)))),
		Type:    types.ContentType(contentType),
		Created: time.Now(),
	}

	return content, nil
}

func (c *DarwinClipboard) Write(content *types.ClipboardContent) error {
	// TODO: Implement pasteboard write
	return nil
}

func (c *DarwinClipboard) HasChanged() bool {
	currentCount := C.getChangeCount()
	if currentCount != c.lastChangeCount {
		c.lastChangeCount = currentCount
		return true
	}
	return false
}
