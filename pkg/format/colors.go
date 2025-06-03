package format

// ANSI color codes
const (
	Reset     = "\033[0m"
	Bold      = "\033[1m"
	Dim       = "\033[2m"
	Italic    = "\033[3m"
	Underline = "\033[4m"

	// Colors
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	Gray    = "\033[37m"
	White   = "\033[97m"

	// Bright colors
	BrightRed     = "\033[91m"
	BrightGreen   = "\033[92m"
	BrightYellow  = "\033[93m"
	BrightBlue    = "\033[94m"
	BrightMagenta = "\033[95m"
	BrightCyan    = "\033[96m"
)

// ColorizeIf applies color only if useColors is true
func ColorizeIf(text, color string, useColors bool) string {
	if !useColors {
		return text
	}
	return color + text + Reset
}

// BoldIf applies bold only if useColors is true
func BoldIf(text string, useColors bool) string {
	if !useColors {
		return text
	}
	return Bold + text + Reset
}

// DimIf applies dim only if useColors is true
func DimIf(text string, useColors bool) string {
	if !useColors {
		return text
	}
	return Dim + text + Reset
} 