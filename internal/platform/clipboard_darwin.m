#import <Cocoa/Cocoa.h>
#include <stdlib.h>

// Get NSPasteboard change count
uint64_t clipman_getChangeCount() {
    NSPasteboard *pb = [NSPasteboard generalPasteboard];
    return (uint64_t)[pb changeCount];
}

// Read text from clipboard (returns malloc'd UTF-8 string, caller must free)
const char* clipman_readText() {
    NSPasteboard *pb = [NSPasteboard generalPasteboard];
    NSString *str = [pb stringForType:NSPasteboardTypeString];
    if (!str) return NULL;
    return strdup([str UTF8String]);
}

// Write text to clipboard (returns true on success)
bool clipman_writeText(const char* text) {
    NSPasteboard *pb = [NSPasteboard generalPasteboard];
    [pb clearContents];
    NSString *str = [NSString stringWithUTF8String:text];
    return [pb setString:str forType:NSPasteboardTypeString];
}

// Read image from clipboard (returns PNG NSData, caller must CFRelease)
NSData* clipman_readImage() {
    NSPasteboard *pb = [NSPasteboard generalPasteboard];
    NSImage *img = [[NSImage alloc] initWithPasteboard:pb];
    if (!img) return nil;
    NSBitmapImageRep *imgRep = [[img representations] objectAtIndex:0];
    return [imgRep representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
}

// Write image to clipboard (expects PNG data, returns true on success)
bool clipman_writeImage(const void* data, size_t len) {
    NSPasteboard *pb = [NSPasteboard generalPasteboard];
    NSData *imgData = [NSData dataWithBytes:data length:len];
    NSImage *img = [[NSImage alloc] initWithData:imgData];
    [pb clearContents];
    return [pb writeObjects:@[img]];
}