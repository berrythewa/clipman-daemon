package format

import (
	"fmt"
	"strings"

	"github.com/berrythewa/clipman-daemon/internal/types"
)

// Formatter is the main formatting orchestrator that delegates to specialized formatters
type Formatter struct {
	options Options
}

// New creates a new formatter with the given options
func New(opts Options) *Formatter {
	return &Formatter{
		options: opts,
	}
}

// NewDefault creates a new formatter with default options
func NewDefault() *Formatter {
	return New(DefaultOptions())
}

// FormatContent formats a single clipboard content entry using the appropriate specialized formatter
func (f *Formatter) FormatContent(content *types.ClipboardContent) string {
	if content == nil {
		return ColorizeIf("No content", Gray, f.options.UseColors)
	}

	var parts []string

	// Header with icon and type
	header := f.formatHeader(content)
	parts = append(parts, header)

	if f.options.Compact {
		// Single line format
		preview := f.formatContentPreview(content, 50)
		parts = append(parts, " "+DimIf(preview, f.options.UseColors))
		return strings.Join(parts, "")
	}

	// Multi-line format
	if f.options.ShowMetadata {
		parts = append(parts, f.formatMetadata(content))
	}

	// Content preview using specialized formatters
	contentStr := f.formatContentData(content)
	if contentStr != "" {
		parts = append(parts, CreateBox("Content", contentStr, f.options))
	}

	return strings.Join(parts, "\n")
}

// FormatContentList formats multiple content entries
func (f *Formatter) FormatContentList(contents []*types.ClipboardContent) string {
	if len(contents) == 0 {
		return ColorizeIf("No clipboard history", Gray, f.options.UseColors)
	}

	var parts []string
	
	// Header
	header := f.formatListHeader(len(contents))
	parts = append(parts, header)
	parts = append(parts, "")

	// Entries
	for i, content := range contents {
		index := fmt.Sprintf("%s[%d]%s", 
			DimIf("", f.options.UseColors), i+1, 
			ColorizeIf("", Reset, f.options.UseColors))
		
		if f.options.Compact {
			line := fmt.Sprintf("%s %s", index, f.FormatContent(content))
			parts = append(parts, line)
		} else {
			parts = append(parts, index)
			parts = append(parts, f.FormatContent(content))
			if i < len(contents)-1 {
				parts = append(parts, CreateSeparator(f.options))
			}
		}
	}

	return strings.Join(parts, "\n")
}

// FormatStats formats clipboard statistics using the stats formatter
func (f *Formatter) FormatStats(stats map[string]interface{}) string {
	return FormatStats(stats, f.options)
}

// formatHeader creates the header with icon and type information
func (f *Formatter) formatHeader(content *types.ClipboardContent) string {
	var parts []string

	// Icon
	if f.options.UseIcons {
		if icon, exists := ContentIcons[content.Type]; exists {
			parts = append(parts, icon)
		}
	}

	// Content type with color
	typeStr := string(content.Type)
	if f.options.UseColors {
		if color, exists := ContentColors[content.Type]; exists {
			typeStr = ColorizeIf(typeStr, color, f.options.UseColors)
		}
	}
	parts = append(parts, typeStr)

	return strings.Join(parts, " ")
}

// formatMetadata creates metadata information
func (f *Formatter) formatMetadata(content *types.ClipboardContent) string {
	var parts []string

	// Content preview instead of hash - much more useful for users
	previewStr := ""
	switch content.Type {
	case types.TypeFile, types.TypeFilePath:
		// For files, show the file path(s)
		previewStr = fmt.Sprintf("Path: %s", FormatFilePreview(content, 60))
	case types.TypeText:
		// For text, show a preview
		preview := FormatTextPreview(content, 40)
		if preview != "" {
			previewStr = fmt.Sprintf("Preview: %s", preview)
		} else {
			previewStr = "Preview: (empty)"
		}
	case types.TypeURL:
		// For URLs, show the URL
		previewStr = fmt.Sprintf("URL: %s", FormatURLPreview(content, 50))
	case types.TypeImage:
		// For images, show image info
		previewStr = FormatImagePreview(content, 40)
	case types.TypeHTML:
		// For HTML, show a preview
		previewStr = fmt.Sprintf("HTML: %s", FormatHTMLPreview(content, 40))
	default:
		// Generic preview for unknown types
		preview := TruncateText(string(content.Data), 40)
		previewStr = fmt.Sprintf("Content: %s", preview)
	}
	
	if previewStr != "" {
		parts = append(parts, DimIf(previewStr, f.options.UseColors))
	}

	// Timestamps
	createdStr := fmt.Sprintf("Created: %s", FormatRelativeTime(content.Created))
	parts = append(parts, DimIf(createdStr, f.options.UseColors))

	// Last occurrence (if multiple)
	if len(content.Occurrences) > 0 {
		lastOccurrence := content.Occurrences[len(content.Occurrences)-1]
		lastStr := fmt.Sprintf("Last seen: %s", FormatRelativeTime(lastOccurrence))
		parts = append(parts, DimIf(lastStr, f.options.UseColors))
	}

	// Size
	size := int64(len(content.Data))
	if size > 0 {
		sizeStr := fmt.Sprintf("Size: %s", FormatSize(size))
		parts = append(parts, DimIf(sizeStr, f.options.UseColors))
	}

	// Occurrences count
	if len(content.Occurrences) > 1 {
		occStr := fmt.Sprintf("Occurrences: %d", len(content.Occurrences))
		parts = append(parts, DimIf(occStr, f.options.UseColors))
	}

	return strings.Join(parts, " • ")
}

// formatContentData delegates to specialized formatters based on content type
func (f *Formatter) formatContentData(content *types.ClipboardContent) string {
	switch content.Type {
	case types.TypeText:
		return FormatText(content, f.options)
	case types.TypeImage:
		return FormatImage(content, f.options)
	case types.TypeFile, types.TypeFilePath:
		return FormatFile(content, f.options)
	case types.TypeURL:
		return FormatURL(content, f.options)
	case types.TypeHTML:
		return FormatHTML(content, f.options)
	default:
		return FormatText(content, f.options)
	}
}

// formatContentPreview creates a brief preview of content using specialized preview formatters
func (f *Formatter) formatContentPreview(content *types.ClipboardContent, maxLen int) string {
	switch content.Type {
	case types.TypeText:
		if len(content.Data) == 0 {
			return "(empty)"
		}
		return FormatTextPreview(content, maxLen)
	case types.TypeImage:
		return FormatImagePreview(content, maxLen)
	case types.TypeFile, types.TypeFilePath:
		return FormatFilePreview(content, maxLen)
	case types.TypeURL:
		return FormatURLPreview(content, maxLen)
	case types.TypeHTML:
		return FormatHTMLPreview(content, maxLen)
	default:
		return TruncateText(string(content.Data), maxLen)
	}
}

// formatListHeader creates the header for content lists
func (f *Formatter) formatListHeader(count int) string {
	title := fmt.Sprintf("📋 Clipboard History (%d entries)", count)
	return ColorizeIf(title, BrightBlue, f.options.UseColors)
}

// Package-level convenience functions

// FormatContent formats a single clipboard content entry with given options
func FormatContent(content *types.ClipboardContent, opts Options) string {
	formatter := New(opts)
	return formatter.FormatContent(content)
}

// FormatContentList formats multiple content entries with given options
func FormatContentList(contents []*types.ClipboardContent, opts Options) string {
	formatter := New(opts)
	return formatter.FormatContentList(contents)
}