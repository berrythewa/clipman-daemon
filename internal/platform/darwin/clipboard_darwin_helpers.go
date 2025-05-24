// +build darwin

package platform

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#include "clipboard_darwin.m"
#include <stdlib.h>
*/
import "C"
import (
	"errors"
	"fmt"
	"unsafe"
)

// --- Change Count ---
func getChangeCount() uint64 {
	return uint64(C.clipman_getChangeCount())
}

// --- Text ---
func readText() (string, error) {
	cstr := C.clipman_readText()
	if cstr == nil {
		return "", errors.New("no text in clipboard")
	}
	defer C.free(unsafe.Pointer(cstr))
	return C.GoString(cstr), nil
}

func writeText(text string) error {
	cstr := C.CString(text)
	defer C.free(unsafe.Pointer(cstr))
	if !C.clipman_writeText(cstr) {
		return errors.New("failed to write text to clipboard")
	}
	return nil
}

// --- HTML ---
func readHTML() (string, error) {
	cstr := C.clipman_readHTML()
	if cstr == nil {
		return "", errors.New("no HTML in clipboard")
	}
	defer C.free(unsafe.Pointer(cstr))
	return C.GoString(cstr), nil
}

func writeHTML(html string) error {
	cstr := C.CString(html)
	defer C.free(unsafe.Pointer(cstr))
	if !C.clipman_writeHTML(cstr) {
		return errors.New("failed to write HTML to clipboard")
	}
	return nil
}

// --- RTF ---
func readRTF() ([]byte, error) {
	var outLen C.size_t
	ptr := C.clipman_readRTF(&outLen)
	if ptr == nil || outLen == 0 {
		return nil, errors.New("no RTF in clipboard")
	}
	defer C.free(unsafe.Pointer(ptr))
	return C.GoBytes(ptr, C.int(outLen)), nil
}

func writeRTF(data []byte) error {
	if len(data) == 0 {
		return errors.New("no RTF data provided")
	}
	if !C.clipman_writeRTF(unsafe.Pointer(&data[0]), C.size_t(len(data))) {
		return errors.New("failed to write RTF to clipboard")
	}
	return nil
}

// --- Image (PNG) ---
func readImage() ([]byte, error) {
	nsdata := C.clipman_readImage()
	if nsdata == nil {
		return nil, errors.New("no image in clipboard")
	}
	length := C.int([nsdata length]);
	ptr := C.NSData_bytes(nsdata);
	if ptr == nil || length == 0 {
		return nil, errors.New("failed to get image data")
	}
	defer C.CFRelease(C.CFTypeRef(nsdata))
	return C.GoBytes(ptr, length), nil
}

func writeImage(data []byte) error {
	if len(data) == 0 {
		return errors.New("no image data provided")
	}
	if !C.clipman_writeImage(unsafe.Pointer(&data[0]), C.size_t(len(data))) {
		return errors.New("failed to write image to clipboard")
	}
	return nil
}

// --- File URLs (single/multiple) ---
func readFileList() ([]string, error) {
	var outCount C.size_t
	cList := C.clipman_readFileList(&outCount)
	if cList == nil || outCount == 0 {
		return nil, errors.New("no file list in clipboard")
	}
	defer C.clipman_freeFileList(cList, outCount)
	result := make([]string, int(outCount))
	cArray := (*[1 << 20]*C.char)(unsafe.Pointer(cList))[:outCount:outCount]
	for i, cstr := range cArray {
		result[i] = C.GoString(cstr)
	}
	return result, nil
}

func writeFileList(paths []string) error {
	count := len(paths)
	if count == 0 {
		return errors.New("no file paths provided")
	}
	cArray := make([]*C.char, count)
	for i, path := range paths {
		cArray[i] = C.CString(path)
		defer C.free(unsafe.Pointer(cArray[i]))
	}
	if !C.clipman_writeFileList((**C.char)(unsafe.Pointer(&cArray[0])), C.size_t(count)) {
		return errors.New("failed to write file list to clipboard")
	}
	return nil
}

// --- URL (as a distinct type) ---
func readURL() (string, error) {
	cstr := C.clipman_readURL()
	if cstr == nil {
		return "", errors.New("no URL in clipboard")
	}
	defer C.free(unsafe.Pointer(cstr))
	return C.GoString(cstr), nil
}

func writeURL(url string) error {
	cstr := C.CString(url)
	defer C.free(unsafe.Pointer(cstr))
	if !C.clipman_writeURL(cstr) {
		return errors.New("failed to write URL to clipboard")
	}
	return nil
}