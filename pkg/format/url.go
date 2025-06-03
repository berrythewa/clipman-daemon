package format

import (
	"github.com/berrythewa/clipman-daemon/internal/types"
)

// FormatURL formats URL content for display
func FormatURL(content *types.ClipboardContent, opts Options) string {
	url := string(content.Data)
	if opts.UseColors {
		return ColorizeIf(url, Underline+Blue, opts.UseColors)
	}
	return url
}

// FormatURLPreview creates a short preview of URL content
func FormatURLPreview(content *types.ClipboardContent, maxLen int) string {
	return TruncateText(string(content.Data), maxLen)
} 