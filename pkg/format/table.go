package format

import (
	"fmt"
	"strings"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/types"
)

// TableColumn represents a column in the table
type TableColumn struct {
	Header string
	Width  int
	Align  string // "left", "right", "center"
}

// FormatContentTable formats multiple content entries in a table format
func FormatContentTable(contents []*types.ClipboardContent, opts Options) string {
	if len(contents) == 0 {
		return ColorizeIf("No clipboard history", Gray, opts.UseColors)
	}

	// Add title header
	var result []string
	title := fmt.Sprintf("📋 Clipboard History (%d entries)", len(contents))
	if opts.UseColors {
		title = ColorizeIf(title, BrightBlue, opts.UseColors)
	}
	result = append(result, title)
	result = append(result, "")

	// Define table columns with better widths
	columns := []TableColumn{
		{Header: "ID", Width: 5, Align: "right"},
		{Header: "Type", Width: 12, Align: "left"},
		{Header: "Preview", Width: 35, Align: "left"},
		{Header: "Tags", Width: 15, Align: "left"},
		{Header: "Size", Width: 8, Align: "right"},
		{Header: "Age", Width: 12, Align: "right"},
	}

	var rows [][]string
	
	// Header row
	headerRow := make([]string, len(columns))
	for i, col := range columns {
		headerRow[i] = col.Header
	}
	rows = append(rows, headerRow)

	// Data rows
	for _, content := range contents {
		row := []string{
			fmt.Sprintf("%d", content.Id),
			formatTypeForTable(content.Type, opts),
			formatPreviewForTable(content, 32), // Reduced width for preview
			formatTagsForTable(content, opts),
			FormatSize(int64(len(content.Data))),
			formatAgeForTable(content),
		}
		rows = append(rows, row)
	}

	// Render the table and combine with header
	tableContent := renderTable(rows, columns, opts)
	result = append(result, tableContent)
	
	return strings.Join(result, "\n")
}

// formatTypeForTable formats content type for table display
func formatTypeForTable(contentType types.ContentType, opts Options) string {
	var result string
	
	// Add icon if enabled
	if opts.UseIcons {
		if icon, exists := ContentIcons[contentType]; exists {
			result += icon + " "
		}
	}
	
	// Add type name with color
	typeName := string(contentType)
	if opts.UseColors {
		if color, exists := ContentColors[contentType]; exists {
			typeName = ColorizeIf(typeName, color, opts.UseColors)
		}
	}
	result += typeName
	
	return result
}

// formatPreviewForTable formats content preview for table display
func formatPreviewForTable(content *types.ClipboardContent, maxLen int) string {
	switch content.Type {
	case types.TypeText:
		preview := FormatTextPreview(content, maxLen)
		if preview == "" {
			return DimIf("(empty)", true)
		}
		return preview
	case types.TypeImage:
		return FormatImagePreview(content, maxLen)
	case types.TypeFile, types.TypeFilePath:
		return FormatFilePreview(content, maxLen)
	case types.TypeURL:
		return FormatURLPreview(content, maxLen)
	default:
		return TruncateText(string(content.Data), maxLen)
	}
}

// formatTagsForTable formats tags for table display
func formatTagsForTable(content *types.ClipboardContent, opts Options) string {
	if len(content.Tags) == 0 {
		return DimIf("(none)", opts.UseColors)
	}
	
	// Join tags with comma, truncate if too long
	tagsText := strings.Join(content.Tags, ", ")
	if len(tagsText) > 13 { // Leave some space for truncation indicator
		tagsText = tagsText[:10] + "..."
	}
	
	if opts.UseColors {
		return ColorizeIf(tagsText, Yellow, opts.UseColors)
	}
	return tagsText
}

// formatAgeForTable formats the age of content for table display
func formatAgeForTable(content *types.ClipboardContent) string {
	var creationTime time.Time
	
	if !content.Created.IsZero() {
		creationTime = content.Created
	} else if len(content.Occurrences) > 0 {
		// Use first occurrence as creation time
		creationTime = content.Occurrences[0]
		for _, occurrence := range content.Occurrences {
			if occurrence.Before(creationTime) {
				creationTime = occurrence
			}
		}
	}
	
	if creationTime.IsZero() {
		return "unknown"
	}
	
	return FormatRelativeTime(creationTime)
}

// renderTable renders the table with proper alignment and borders
func renderTable(rows [][]string, columns []TableColumn, opts Options) string {
	if len(rows) == 0 {
		return ""
	}

	var result []string
	
	// Calculate actual column widths based on content
	actualWidths := make([]int, len(columns))
	for i, col := range columns {
		actualWidths[i] = col.Width
		// Check if any cell content is wider
		for _, row := range rows {
			if i < len(row) {
				// Remove ANSI codes for width calculation
				cleanContent := removeAnsiCodes(row[i])
				if len(cleanContent) > actualWidths[i] {
					actualWidths[i] = len(cleanContent)
				}
			}
		}
	}

	// Create table header
	header := createTableRow(rows[0], actualWidths, columns, opts, true)
	result = append(result, header)
	
	// Create separator line
	separator := createSeparatorLine(actualWidths, opts)
	result = append(result, separator)
	
	// Create data rows
	for i := 1; i < len(rows); i++ {
		row := createTableRow(rows[i], actualWidths, columns, opts, false)
		result = append(result, row)
	}

	return strings.Join(result, "\n")
}

// createTableRow creates a formatted table row
func createTableRow(row []string, widths []int, columns []TableColumn, opts Options, isHeader bool) string {
	var cells []string
	
	for i, content := range row {
		if i >= len(widths) {
			break
		}
		
		// Apply alignment
		align := "left"
		if i < len(columns) {
			align = columns[i].Align
		}
		
		// Format cell content
		cell := formatCell(content, widths[i], align, opts, isHeader)
		cells = append(cells, cell)
	}
	
	// Join with borders
	border := "|"
	if opts.UseColors {
		border = DimIf("|", opts.UseColors)
	}
	
	return border + " " + strings.Join(cells, " " + border + " ") + " " + border
}

// formatCell formats a single table cell with proper alignment
func formatCell(content string, width int, align string, opts Options, isHeader bool) string {
	// Clean content for width calculation (remove ANSI codes)
	cleanContent := removeAnsiCodes(content)
	contentWidth := len(cleanContent)
	
	if contentWidth >= width {
		return TruncateText(content, width)
	}
	
	padding := width - contentWidth
	
	if isHeader && opts.UseColors {
		content = BoldIf(content, opts.UseColors)
	}
	
	switch align {
	case "right":
		return strings.Repeat(" ", padding) + content
	case "center":
		leftPad := padding / 2
		rightPad := padding - leftPad
		return strings.Repeat(" ", leftPad) + content + strings.Repeat(" ", rightPad)
	default: // "left"
		return content + strings.Repeat(" ", padding)
	}
}

// createSeparatorLine creates a separator line for the table
func createSeparatorLine(widths []int, opts Options) string {
	var parts []string
	
	for _, width := range widths {
		parts = append(parts, strings.Repeat("-", width))
	}
	
	separator := "+-" + strings.Join(parts, "-+-") + "-+"
	if opts.UseColors {
		separator = DimIf(separator, opts.UseColors)
	}
	
	return separator
}

// removeAnsiCodes removes ANSI color codes for width calculation
func removeAnsiCodes(text string) string {
	// Simple implementation - remove common ANSI sequences
	result := text
	ansiCodes := []string{
		"\033[0m", "\033[1m", "\033[2m", "\033[3m", "\033[4m",
		"\033[31m", "\033[32m", "\033[33m", "\033[34m", "\033[35m", "\033[36m", "\033[37m",
		"\033[91m", "\033[92m", "\033[93m", "\033[94m", "\033[95m", "\033[96m", "\033[97m",
	}
	
	for _, code := range ansiCodes {
		result = strings.ReplaceAll(result, code, "")
	}
	
	return result
}
