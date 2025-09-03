package format

import "github.com/berrythewa/clipman-daemon/internal/types"

// DisplayMode represents different display formats
type DisplayMode string

const (
	DisplayDefault DisplayMode = "default" // Multi-line format with full details
	DisplayCompact DisplayMode = "compact" // Single-line format
	DisplayTable   DisplayMode = "table"   // Tabular format
	DisplayGrid    DisplayMode = "grid"    // Grid-based layout
)

// Options controls formatting behavior
type Options struct {
	UseColors    bool        // Enable ANSI color codes
	UseIcons     bool        // Show Unicode icons
	MaxWidth     int         // Max content width (0 = no limit)
	MaxLines     int         // Max content lines (0 = no limit)
	ShowMetadata bool        // Show hash, timestamps, etc.
	Compact      bool        // Use compact single-line format (deprecated: use DisplayMode)
	DisplayMode  DisplayMode // Display format mode
}

// DefaultOptions returns sensible defaults
func DefaultOptions() Options {
	return Options{
		UseColors:    true,
		UseIcons:     true,
		MaxWidth:     80,
		MaxLines:     10,
		ShowMetadata: true,
		Compact:      false,
	}
}

// CompactOptions returns options for compact single-line display
func CompactOptions() Options {
	opts := DefaultOptions()
	opts.Compact = true
	opts.ShowMetadata = false
	opts.MaxLines = 1
	opts.DisplayMode = DisplayCompact
	return opts
}

// TableOptions returns options for tabular display
func TableOptions() Options {
	opts := DefaultOptions()
	opts.DisplayMode = DisplayTable
	opts.MaxLines = 1
	opts.ShowMetadata = true
	return opts
}

// GridOptions returns options for grid-based display
func GridOptions() Options {
	opts := DefaultOptions()
	opts.DisplayMode = DisplayGrid
	opts.ShowMetadata = true
	return opts
}

// ContentIcons maps content types to Unicode icons
var ContentIcons = map[types.ContentType]string{
	types.TypeText:     "📝",
	types.TypeImage:    "🖼️",
	types.TypeFile:     "📎",
	types.TypeFilePath: "📁",
	types.TypeURL:      "🔗",
	types.TypeHTML:     "🌐",
	types.TypeHTMLText: "📄",
	types.TypeRTF:      "📄",
}

// ContentColors maps content types to colors
var ContentColors = map[types.ContentType]string{
	types.TypeText:     Cyan,
	types.TypeImage:    Magenta,
	types.TypeFile:     Yellow,
	types.TypeFilePath: BrightYellow,
	types.TypeURL:      Blue,
	types.TypeHTML:     Green,
	types.TypeHTMLText: BrightCyan,
	types.TypeRTF:      Gray,
}
