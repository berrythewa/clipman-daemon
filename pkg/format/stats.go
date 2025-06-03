package format

import (
	"fmt"
	"strings"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/types"
)

// FormatStats formats clipboard statistics for display
func FormatStats(stats map[string]interface{}, opts Options) string {
	var parts []string

	// Title
	title := "📊 Clipboard Statistics"
	if opts.UseColors {
		title = ColorizeIf("📊 Clipboard Statistics", BrightBlue, opts.UseColors)
	}
	parts = append(parts, title)
	parts = append(parts, "")

	// Total entries
	if total, ok := stats["total_entries"].(int); ok {
		parts = append(parts, formatStatLine("Total entries", fmt.Sprintf("%d", total), opts))
	}

	// Total size
	if size, ok := stats["total_size"].(int64); ok {
		parts = append(parts, formatStatLine("Total size", FormatSize(size), opts))
	}

	// Time range
	if oldest, ok := stats["oldest_entry"].(time.Time); ok {
		parts = append(parts, formatStatLine("Oldest entry", FormatRelativeTime(oldest), opts))
	}
	if newest, ok := stats["newest_entry"].(time.Time); ok {
		parts = append(parts, formatStatLine("Newest entry", FormatRelativeTime(newest), opts))
	}

	// Entries by type
	if typeStats, ok := stats["entries_by_type"].(map[string]int); ok && len(typeStats) > 0 {
		parts = append(parts, "")
		parts = append(parts, formatSubHeader("Entries by type", opts))
		for typ, count := range typeStats {
			contentType := types.ContentType(typ)
			icon := ""
			if opts.UseIcons {
				if i, exists := ContentIcons[contentType]; exists {
					icon = i + " "
				}
			}
			
			line := fmt.Sprintf("  %s%s: %d", icon, typ, count)
			if opts.UseColors {
				if color, exists := ContentColors[contentType]; exists {
					line = fmt.Sprintf("  %s%s%s %s: %d", color, icon, Reset, typ, count)
				}
			}
			
			parts = append(parts, line)
		}
	}

	return strings.Join(parts, "\n")
}

// formatStatLine formats a statistics line with label and value
func formatStatLine(label, value string, opts Options) string {
	if opts.UseColors {
		return fmt.Sprintf("  %s%s:%s %s", BrightCyan, label, Reset, value)
	}
	return fmt.Sprintf("  %s: %s", label, value)
}

// formatSubHeader formats a section subheader
func formatSubHeader(title string, opts Options) string {
	return ColorizeIf(title, BrightBlue, opts.UseColors)
} 