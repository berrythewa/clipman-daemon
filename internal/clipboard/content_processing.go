package clipboard

import (
	"bytes"
	"regexp"

	"github.com/berrythewa/clipman-daemon/internal/types"
)

type ContentFilter func(*types.ClipboardContent) bool
type ContentTransformer func(*types.ClipboardContent) *types.ClipboardContent

// ContentProcessor combines filtering and transformation
type ContentProcessor struct {
	Filters      []ContentFilter
	Transformers []ContentTransformer
}

func NewContentProcessor() *ContentProcessor {
	return &ContentProcessor{
		Filters:      make([]ContentFilter, 0),
		Transformers: make([]ContentTransformer, 0),
	}
}

func (cp *ContentProcessor) AddFilter(filter ContentFilter) {
	cp.Filters = append(cp.Filters, filter)
}

func (cp *ContentProcessor) AddTransformer(transformer ContentTransformer) {
	cp.Transformers = append(cp.Transformers, transformer)
}

func (cp *ContentProcessor) Process(content *types.ClipboardContent) *types.ClipboardContent {
	for _, filter := range cp.Filters {
		if !filter(content) {
			return nil // Content filtered out
		}
	}

	for _, transformer := range cp.Transformers {
		content = transformer(content)
	}

	return content
}

// Example filters and transformers

func LengthFilter(maxLength int) ContentFilter {
	return func(content *types.ClipboardContent) bool {
		return len(content.Data) <= maxLength
	}
}

func RegexFilter(pattern string) ContentFilter {
	regex := regexp.MustCompile(pattern)
	return func(content *types.ClipboardContent) bool {
		return regex.Match(content.Data)
	}
}

func TrimTransformer() ContentTransformer {
	return func(content *types.ClipboardContent) *types.ClipboardContent {
		content.Data = bytes.TrimSpace(content.Data)
		return content
	}
}

func LowercaseTransformer() ContentTransformer {
	return func(content *types.ClipboardContent) *types.ClipboardContent {
		content.Data = bytes.ToLower(content.Data)
		return content
	}
}
