package format

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

// FormatSize formats a byte count as a human-readable string
func FormatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// FormatRelativeTime formats a time as a human-readable relative string
func FormatRelativeTime(t time.Time) string {
	now := time.Now()
	diff := now.Sub(t)

	if diff < time.Minute {
		return "just now"
	} else if diff < time.Hour {
		return fmt.Sprintf("%d minutes ago", int(diff.Minutes()))
	} else if diff < 24*time.Hour {
		return fmt.Sprintf("%d hours ago", int(diff.Hours()))
	} else if diff < 7*24*time.Hour {
		return fmt.Sprintf("%d days ago", int(diff.Hours()/24))
	} else {
		return t.Format("Jan 2, 2006")
	}
}

// TruncateText truncates text to maxLen runes with ellipsis
func TruncateText(text string, maxLen int) string {
	if maxLen <= 0 {
		return text
	}

	if utf8.RuneCountInString(text) <= maxLen {
		return text
	}

	runes := []rune(text)
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}

	return string(runes[:maxLen-3]) + "..."
}

// TruncateLines truncates text to maxLines with summary
func TruncateLines(text string, maxLines int) string {
	if maxLines <= 0 {
		return text
	}

	lines := strings.Split(text, "\n")
	if len(lines) <= maxLines {
		return text
	}

	truncated := strings.Join(lines[:maxLines], "\n")
	remaining := len(lines) - maxLines
	return truncated + fmt.Sprintf("\n... (%d more lines)", remaining)
}

// IndentText indents each line with the given prefix
func IndentText(text, prefix string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		lines[i] = prefix + line
	}
	return strings.Join(lines, "\n")
}

// CreateBox creates a simple text box with title
func CreateBox(title, content string, opts Options) string {
	if content == "" {
		return ""
	}

	var parts []string

	// Box title
	titleStr := "▼ " + title
	if opts.UseColors {
		titleStr = DimIf(titleStr, opts.UseColors)
	}
	parts = append(parts, titleStr)

	// Indent content
	indented := IndentText(content, "  ")
	parts = append(parts, indented)

	return strings.Join(parts, "\n")
}

// CreateSeparator creates a visual separator line
func CreateSeparator(opts Options) string {
	sep := "────────────────────────────────────────"
	return DimIf(sep, opts.UseColors)
} 