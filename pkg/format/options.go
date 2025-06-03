package format

import "github.com/berrythewa/clipman-daemon/internal/types"

// Options controls formatting behavior
type Options struct {
	UseColors    bool
	UseIcons     bool
	MaxWidth     int  // Max content width (0 = no limit)
	MaxLines     int  // Max content lines (0 = no limit)
	ShowMetadata bool // Show hash, timestamps, etc.
	Compact      bool // Use compact single-line format
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
	types.TypeRTF:      Gray,
} 