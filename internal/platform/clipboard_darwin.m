#import <Cocoa/Cocoa.h>
#include <stdlib.h>

/*
Notes
    .All returned C strings and file lists are malloc'd; you must free them in Go after use.
    .For file lists, use clipman_freeFileList to free the array and its contents.
    This covers all major clipboard data types you’ll need for parity with Linux 
    and for future distributed clipboard featur
*/

// --- Change Count ---
uint64_t clipman_getChangeCount() {
    NSPasteboard *pb = [NSPasteboard generalPasteboard];
    return (uint64_t)[pb changeCount];
}

// --- Text ---
const char* clipman_readText() {
    NSPasteboard *pb = [NSPasteboard generalPasteboard];
    NSString *str = [pb stringForType:NSPasteboardTypeString];
    if (!str) return NULL;
    return strdup([str UTF8String]);
}
bool clipman_writeText(const char* text) {
    NSPasteboard *pb = [NSPasteboard generalPasteboard];
    [pb clearContents];
    NSString *str = [NSString stringWithUTF8String:text];
    return [pb setString:str forType:NSPasteboardTypeString];
}

// --- HTML ---
const char* clipman_readHTML() {
    NSPasteboard *pb = [NSPasteboard generalPasteboard];
    NSString *str = [pb stringForType:NSPasteboardTypeHTML];
    if (!str) return NULL;
    return strdup([str UTF8String]);
}
bool clipman_writeHTML(const char* html) {
    NSPasteboard *pb = [NSPasteboard generalPasteboard];
    [pb clearContents];
    NSString *str = [NSString stringWithUTF8String:html];
    return [pb setString:str forType:NSPasteboardTypeHTML];
}

// --- RTF ---
const void* clipman_readRTF(size_t* outLen) {
    NSPasteboard *pb = [NSPasteboard generalPasteboard];
    NSData *data = [pb dataForType:NSPasteboardTypeRTF];
    if (!data) return NULL;
    *outLen = [data length];
    void* buf = malloc(*outLen);
    memcpy(buf, [data bytes], *outLen);
    return buf;
}
bool clipman_writeRTF(const void* data, size_t len) {
    NSPasteboard *pb = [NSPasteboard generalPasteboard];
    [pb clearContents];
    NSData *rtfData = [NSData dataWithBytes:data length:len];
    return [pb setData:rtfData forType:NSPasteboardTypeRTF];
}

// --- Image (PNG) ---
NSData* clipman_readImage() {
    NSPasteboard *pb = [NSPasteboard generalPasteboard];
    NSImage *img = [[NSImage alloc] initWithPasteboard:pb];
    if (!img) return nil;
    NSBitmapImageRep *imgRep = [[img representations] objectAtIndex:0];
    return [imgRep representationUsingType:NSBitmapImageFileTypePNG properties:@{}];
}
bool clipman_writeImage(const void* data, size_t len) {
    NSPasteboard *pb = [NSPasteboard generalPasteboard];
    NSData *imgData = [NSData dataWithBytes:data length:len];
    NSImage *img = [[NSImage alloc] initWithData:imgData];
    [pb clearContents];
    return [pb writeObjects:@[img]];
}

// --- File URLs (single/multiple) ---
char** clipman_readFileList(size_t* outCount) {
    NSPasteboard *pb = [NSPasteboard generalPasteboard];
    NSArray<NSURL*> *urls = [pb readObjectsForClasses:@[[NSURL class]] options:@{NSPasteboardURLReadingFileURLsOnlyKey: @YES}];
    if (!urls || [urls count] == 0) {
        *outCount = 0;
        return NULL;
    }
    *outCount = [urls count];
    char** result = (char**)malloc(sizeof(char*) * (*outCount));
    for (NSUInteger i = 0; i < [urls count]; ++i) {
        NSString *path = [[urls objectAtIndex:i] path];
        result[i] = strdup([path UTF8String]);
    }
    return result;
}
void clipman_freeFileList(char** list, size_t count) {
    for (size_t i = 0; i < count; ++i) {
        free(list[i]);
    }
    free(list);
}
bool clipman_writeFileList(const char** paths, size_t count) {
    NSPasteboard *pb = [NSPasteboard generalPasteboard];
    [pb clearContents];
    NSMutableArray *urlObjs = [NSMutableArray arrayWithCapacity:count];
    for (size_t i = 0; i < count; ++i) {
        NSString *path = [NSString stringWithUTF8String:paths[i]];
        NSURL *url = [NSURL fileURLWithPath:path];
        [urlObjs addObject:url];
    }
    return [pb writeObjects:urlObjs];
}

// --- URL (as a distinct type) ---
const char* clipman_readURL() {
    NSPasteboard *pb = [NSPasteboard generalPasteboard];
    NSString *str = [pb stringForType:NSPasteboardTypeURL];
    if (!str) return NULL;
    return strdup([str UTF8String]);
}
bool clipman_writeURL(const char* url) {
    NSPasteboard *pb = [NSPasteboard generalPasteboard];
    [pb clearContents];
    NSString *str = [NSString stringWithUTF8String:url];
    NSURL *nsurl = [NSURL URLWithString:str];
    return [pb writeObjects:@[nsurl]];
}