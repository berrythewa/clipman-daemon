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

// IsPasswordField checks if the content appears to come from a password field
func IsPasswordField(html string, plainText string) bool {
	if html == "" {
		return false
	}
	
	// Check for password field indicators in HTML
	passwordIndicators := []string{
		`type="password"`,
		`type='password'`,
		`class="password"`,
		`class='password'`,
		`id="password"`,
		`id='password'`,
		`name="password"`,
		`name='password'`,
		`data-type="password"`,
		`data-type='password'`,
		`password-field`,
		`password-input`,
		`pwd-field`,
		`pwd-input`,
	}
	
	htmlLower := strings.ToLower(html)
	for _, indicator := range passwordIndicators {
		if strings.Contains(htmlLower, indicator) {
			return true
		}
	}
	
	// Check for password-related CSS classes
	passwordClasses := []string{
		`class="password`,
		`class='password`,
		`class="pwd`,
		`class='pwd`,
		`class="secret`,
		`class='secret`,
		`class="hidden`,
		`class='hidden`,
	}
	
	for _, class := range passwordClasses {
		if strings.Contains(htmlLower, class) {
			return true
		}
	}
	
	// Check for password-related IDs
	passwordIDs := []string{
		`id="password`,
		`id='password`,
		`id="pwd`,
		`id='pwd`,
		`id="pass`,
		`id='pass`,
		`id="secret`,
		`id='secret`,
	}
	
	for _, id := range passwordIDs {
		if strings.Contains(htmlLower, id) {
			return true
		}
	}
	
	// Check for password-related names
	passwordNames := []string{
		`name="password`,
		`name='password`,
		`name="pwd`,
		`name='pwd`,
		`name="pass`,
		`name='pass`,
		`name="secret`,
		`name='secret`,
	}
	
	for _, name := range passwordNames {
		if strings.Contains(htmlLower, name) {
			return true
		}
	}
	
	return false
}

// IsPasswordContent checks if the plain text content looks like a password
func IsPasswordContent(text string) bool {
	if text == "" {
		return false
	}
	
	// Remove whitespace
	text = strings.TrimSpace(text)
	
	// Check length (typical password lengths)
	if len(text) < 4 || len(text) > 128 {
		return false
	}
	
	// Check for common password patterns
	passwordPatterns := []string{
		`^[a-zA-Z0-9!@#$%^&*()_+\-=\[\]{};':"\\|,.<>\/?]{4,}$`, // Alphanumeric + symbols
		`^[a-zA-Z0-9]{8,}$`, // Alphanumeric only
		`^[a-zA-Z]{8,}$`,    // Letters only
		`^[0-9]{4,}$`,       // Numbers only
	}
	
	for _, pattern := range passwordPatterns {
		matched, _ := regexp.MatchString(pattern, text)
		if matched {
			return true
		}
	}
	
	// Check for high entropy (randomness) - simple heuristic
	uniqueChars := make(map[rune]bool)
	for _, char := range text {
		uniqueChars[char] = true
	}
	
	// If more than 50% of characters are unique, likely a password
	if float64(len(uniqueChars))/float64(len(text)) > 0.5 {
		return true
	}
	
	return false
}

// DetectPasswordContent detects if content is from a password field
func DetectPasswordContent(html string, plainText string) bool {
	// First check HTML context (most reliable)
	if IsPasswordField(html, plainText) {
		return true
	}
	
	// Then check content patterns (fallback)
	if IsPasswordContent(plainText) {
		return true
	}
	
	return false
} 