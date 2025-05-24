//go:build linux
// +build linux

package platform

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/berrythewa/clipman-daemon/pkg/utils"
)

// detectContent determines the content type and creates a ClipboardContent
func detectContent(text string) (*types.ClipboardContent, error) {
	contentType := detectContentType(text)
	return utils.NewClipboardContent(contentType, []byte(text)), nil
}

// detectContentType attempts to determine the content type
func detectContentType(text string) types.ContentType {
	if text == "" {
		return types.TypeText
	}

	// Check for JSON-encoded file list
	var files []string
	if err := json.Unmarshal([]byte(text), &files); err == nil && len(files) > 0 {
		// Verify first file exists
		if _, err := os.Stat(files[0]); err == nil {
			return types.TypeFile
		}
	}

	// Check for single file path
	if _, err := os.Stat(text); err == nil {
		return types.TypeFilePath
	}

	// Check for URL
	if isURL(text) {
		return types.TypeURL
	}

	// Check for HTML
	if isHTML(text) {
		return types.TypeHTML
	}

	// Check for RTF
	if isRTF(text) {
		return types.TypeRTF
	}

	return types.TypeText
}

// isURL checks if the text appears to be a URL
func isURL(text string) bool {
	urlPrefixes := []string{
		"http://",
		"https://",
		"ftp://",
		"sftp://",
		"file://",
	}

	text = strings.TrimSpace(text)
	for _, prefix := range urlPrefixes {
		if strings.HasPrefix(text, prefix) {
			return true
		}
	}
	return false
}

// isHTML checks if the text appears to be HTML
func isHTML(text string) bool {
	text = strings.TrimSpace(text)
	if len(text) < 6 {
		return false
	}

	htmlStarts := []string{
		"<html>",
		"<!DOCTYPE",
		"<!doctype",
		"<HTML>",
	}

	for _, start := range htmlStarts {
		if strings.HasPrefix(text, start) {
			return true
		}
	}
	return false
}

// isRTF checks if the text appears to be RTF
func isRTF(text string) bool {
	text = strings.TrimSpace(text)
	return len(text) > 5 && strings.HasPrefix(text, "{\\rtf")
} 