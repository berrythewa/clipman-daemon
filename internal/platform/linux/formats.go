//go:build linux
// +build linux

package platform

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/berrythewa/clipman-daemon/pkg/utils"
)

// DetectContent determines the content type and creates a ClipboardContent
func DetectContent(text string) (*types.ClipboardContent, error) {
	contentType := DetectContentType(text)
	return utils.NewClipboardContent(contentType, []byte(text)), nil
}

// DetectContentType attempts to determine the content type
func DetectContentType(text string) types.ContentType {
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
	if IsURL(text) {
		return types.TypeURL
	}

	// Check for HTML
	if IsHTML(text) {
		return types.TypeHTML
	}

	// Check for RTF
	if IsRTF(text) {
		return types.TypeRTF
	}

	return types.TypeText
}

// DetectContentWithHTMLTextExtraction detects content type and optionally extracts text from HTML
func DetectContentWithHTMLTextExtraction(text string, extractHTMLText bool) (*types.ClipboardContent, error) {
	if text == "" {
		return utils.NewClipboardContent(types.TypeText, []byte(text)), nil
	}

	// If HTML text extraction is requested and content is HTML
	if extractHTMLText && IsHTML(text) {
		extractedText := ExtractTextFromHTML(text)
		if extractedText != text { // Only if extraction actually changed something
			return utils.NewClipboardContent(types.TypeHTMLText, []byte(extractedText)), nil
		}
	}

	// Use normal detection
	contentType := DetectContentType(text)
	return utils.NewClipboardContent(contentType, []byte(text)), nil
}

// ExtractTextFromHTML extracts plain text from HTML content
func ExtractTextFromHTML(html string) string {
	if html == "" {
		return ""
	}

	// Remove HTML comments
	html = regexp.MustCompile(`<!--[\s\S]*?-->`).ReplaceAllString(html, "")
	
	// Remove script and style tags and their content
	html = regexp.MustCompile(`<script[\s\S]*?</script>`).ReplaceAllString(html, "")
	html = regexp.MustCompile(`<style[\s\S]*?</style>`).ReplaceAllString(html, "")
	
	// Replace common HTML entities
	html = strings.ReplaceAll(html, "&nbsp;", " ")
	html = strings.ReplaceAll(html, "&amp;", "&")
	html = strings.ReplaceAll(html, "&lt;", "<")
	html = strings.ReplaceAll(html, "&gt;", ">")
	html = strings.ReplaceAll(html, "&quot;", "\"")
	html = strings.ReplaceAll(html, "&#39;", "'")
	
	// Remove all HTML tags
	html = regexp.MustCompile(`<[^>]*>`).ReplaceAllString(html, "")
	
	// Normalize whitespace
	html = regexp.MustCompile(`\s+`).ReplaceAllString(html, " ")
	
	// Trim whitespace
	html = strings.TrimSpace(html)
	
	return html
}

// IsURL checks if the text appears to be a URL
func IsURL(text string) bool {
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

// IsHTML checks if the text appears to be HTML
func IsHTML(text string) bool {
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
	
	// Also check for HTML tags anywhere in the content
	htmlTagPattern := regexp.MustCompile(`<[^>]+>`)
	return htmlTagPattern.MatchString(text)
}

// IsRTF checks if the text appears to be RTF
func IsRTF(text string) bool {
	text = strings.TrimSpace(text)
	return len(text) > 5 && strings.HasPrefix(text, "{\\rtf")
} 