package clipboard

import (
	"bytes"
	"encoding/json"
	"os"
	"regexp"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"go.uber.org/zap"
)

type ContentFilter func(*types.ClipboardContent) bool
type ContentTransformer func(*types.ClipboardContent) *types.ClipboardContent

// ContentProcessor combines filters and transformations
type ContentProcessor struct {
	filters      []ContentFilter
	transformers []ContentTransformer
	logger       *zap.Logger
	MaxSizeBytes int64 // 100MB default
}

// NewContentProcessor creates a new content processor
func NewContentProcessor() *ContentProcessor {
	return &ContentProcessor{
		filters:      []ContentFilter{},
		transformers: []ContentTransformer{},
		MaxSizeBytes: 100 * 1024 * 1024, // 100MB default
	}
}

// SetLogger sets the logger for the content processor
func (c *ContentProcessor) SetLogger(logger *zap.Logger) {
	c.logger = logger
}

// SetMaxSize sets the maximum size for content processing in bytes
func (c *ContentProcessor) SetMaxSize(maxSizeBytes int64) {
	c.MaxSizeBytes = maxSizeBytes
}

func (cp *ContentProcessor) AddFilter(filter ContentFilter) {
	cp.filters = append(cp.filters, filter)
}

func (cp *ContentProcessor) AddTransformer(transformer ContentTransformer) {
	cp.transformers = append(cp.transformers, transformer)
}

// Process applies all filters and transformations to the content
func (c *ContentProcessor) Process(content *types.ClipboardContent) *types.ClipboardContent {
	if content == nil {
		return nil
	}

	// Apply size filter first
	if len, ok := contentSize(content); ok && len > c.MaxSizeBytes {
		if c.logger != nil {
			c.logger.Debug("Content exceeds maximum size",
				zap.Int64("max_size_bytes", c.MaxSizeBytes),
				zap.Int64("content_size_bytes", len),
				zap.String("content_type", string(content.Type)))
		}
		return nil
	}

	// Apply type-specific processing first
	switch content.Type {
	case types.TypeText, types.TypeString:
		content = c.processTextContent(content)
	case types.TypeURL:
		content = c.processUrlContent(content)
	case types.TypeFilePath:
		content = c.processFilePathContent(content)
	case types.TypeFile:
		content = c.processFileListContent(content)
	case types.TypeImage:
		// Images are already binary, leave as is
	}

	// Apply all transformers
	for _, transformer := range c.transformers {
		content = transformer(content)
		if content == nil {
			return nil
		}
	}

	// Apply all filters
	for _, filter := range c.filters {
		if !filter(content) {
			return nil
		}
	}

	return content
}

// contentSize attempts to determine the size of the content data
func contentSize(content *types.ClipboardContent) (int64, bool) {
	return int64(len(content.Data)), true
}

// processTextContent applies transformations specific to text content
func (cp *ContentProcessor) processTextContent(content *types.ClipboardContent) *types.ClipboardContent {
	// Trim whitespace for text content
	content.Data = bytes.TrimSpace(content.Data)
	return content
}

// processUrlContent processes URL content
func (cp *ContentProcessor) processUrlContent(content *types.ClipboardContent) *types.ClipboardContent {
	// Trim whitespace for URLs too
	content.Data = bytes.TrimSpace(content.Data)
	return content
}

// processFilePathContent processes file path content
// This implementation keeps just the file path, but could be extended to include
// actual file content if needed
func (cp *ContentProcessor) processFilePathContent(content *types.ClipboardContent) *types.ClipboardContent {
	filePath := string(content.Data)
	
	// Validate the file exists
	info, err := os.Stat(filePath)
	if err != nil {
		if cp.logger != nil {
			cp.logger.Debug("File path not accessible",
				zap.String("path", filePath),
				zap.Error(err))
		}
		// Keep the path even if we can't access the file
		return content
	}
	
	// If it's a directory, keep as is
	if info.IsDir() {
		return content
	}
	
	// For now, we just keep the path - we're not reading file contents
	// This can be modified to read file content if needed
	return content
}

// processFileListContent processes a list of file paths
// Similar to file path processing, this keeps just the paths
func (cp *ContentProcessor) processFileListContent(content *types.ClipboardContent) *types.ClipboardContent {
	var filePaths []string
	
	// Unmarshal the list of paths
	if err := json.Unmarshal(content.Data, &filePaths); err != nil {
		if cp.logger != nil {
			cp.logger.Debug("Failed to unmarshal file list",
				zap.Error(err))
		}
		return content
	}
	
	// Validate each file exists
	validPaths := make([]string, 0, len(filePaths))
	for _, path := range filePaths {
		// Check file exists and is accessible
		_, err := os.Stat(path)
		if err == nil {
			// File exists and is accessible
			validPaths = append(validPaths, path)
		} else if cp.logger != nil {
			cp.logger.Debug("Skipping inaccessible file",
				zap.String("path", path),
				zap.Error(err))
		}
	}
	
	// If all paths were invalid, keep original content
	if len(validPaths) == 0 && len(filePaths) > 0 {
		return content
	}
	
	// Marshal the valid paths back to JSON
	newData, err := json.Marshal(validPaths)
	if err != nil {
		if cp.logger != nil {
			cp.logger.Debug("Failed to marshal valid file paths",
				zap.Error(err))
		}
		return content
	}
	
	// Return updated content
	content.Data = newData
	return content
}

// Helper functions for filters and transformers

// SizeFilter creates a filter that excludes content larger than the specified size
func SizeFilter(maxSizeBytes int64) ContentFilter {
	return func(content *types.ClipboardContent) bool {
		return int64(len(content.Data)) <= maxSizeBytes
	}
}

// TypeFilter creates a filter that only allows specific content types
func TypeFilter(allowedTypes ...types.ContentType) ContentFilter {
	return func(content *types.ClipboardContent) bool {
		for _, allowedType := range allowedTypes {
			if content.Type == allowedType {
				return true
			}
		}
		return false
	}
}

// ExcludeTypeFilter creates a filter that excludes specific content types
func ExcludeTypeFilter(excludedTypes ...types.ContentType) ContentFilter {
	return func(content *types.ClipboardContent) bool {
		for _, excludedType := range excludedTypes {
			if content.Type == excludedType {
				return false
			}
		}
		return true
	}
}

// RegexFilter creates a filter based on a regular expression
func RegexFilter(pattern string) ContentFilter {
	regex := regexp.MustCompile(pattern)
	return func(content *types.ClipboardContent) bool {
		return regex.Match(content.Data)
	}
}

// TrimTransformer creates a transformer that trims whitespace from text content
func TrimTransformer() ContentTransformer {
	return func(content *types.ClipboardContent) *types.ClipboardContent {
		// Only trim text and URL content, leave files and images untouched
		if content.Type == types.TypeText || content.Type == types.TypeURL {
			content.Data = bytes.TrimSpace(content.Data)
		}
		return content
	}
}

// LowercaseTransformer creates a transformer that converts text to lowercase
func LowercaseTransformer() ContentTransformer {
	return func(content *types.ClipboardContent) *types.ClipboardContent {
		// Only transform if it's text content
		if content.Type == types.TypeText || content.Type == types.TypeString {
			content.Data = bytes.ToLower(content.Data)
		}
		return content
	}
}
