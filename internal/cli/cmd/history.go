package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/storage"
	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/spf13/cobra"
)

var (
	// History filtering flags
	limit      int
	since      string
	before     string
	itemType   string
	reverse    bool
	minSize    int
	contentMaxSize  int
	jsonOutput bool
	dumpAll    bool
)

// historyCmd represents the history command
var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Display and filter clipboard history",
	Long: `Display clipboard history with various filtering options.
You can filter by time range, content type, size, and more.

Examples:
  # Show the last 5 items
  clipmand history --limit 5

  # Show all text items in reverse order (newest first)
  clipmand history --type text --reverse

  # Show items from a specific time range
  clipmand history --since 2023-01-01T00:00:00Z --before 2023-01-31T23:59:59Z

  # Show items larger than a specific size
  clipmand history --min-size 1024
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Initialize storage
		dbPath := filepath.Join(cfg.DataDir, "clipboard.db")
		storageConfig := storage.StorageConfig{
			DBPath:   dbPath,
			DeviceID: cfg.DeviceID,
			Logger:   logger,
		}
		
		store, err := storage.NewBoltStorage(storageConfig)
		if err != nil {
			logger.Error("Failed to initialize storage", "error", err)
			return err
		}
		defer store.Close()
		
		// Configure history options
		historyOptions := config.HistoryOptions{
			Limit:   limit,
			Reverse: reverse,
			MinSize: minSize,
		}
		
		if contentMaxSize > 0 {
			historyOptions.MaxSize = contentMaxSize
		}
		
		// Check if --dump-all flag is set
		if dumpAll {
			logger.Info("Dumping complete clipboard history")
			return store.LogCompleteHistory(config.HistoryOptions{})
		}
		
		// Parse time filters
		if since != "" {
			sinceTime, err := time.Parse(time.RFC3339, since)
			if err != nil {
				return fmt.Errorf("invalid time format for --since: %v", err)
			}
			historyOptions.Since = sinceTime
		}
		
		if before != "" {
			beforeTime, err := time.Parse(time.RFC3339, before)
			if err != nil {
				return fmt.Errorf("invalid time format for --before: %v", err)
			}
			historyOptions.Before = beforeTime
		}
		
		// Parse content type filter
		if itemType != "" {
			switch itemType {
			case "text":
				historyOptions.ContentType = types.TypeText
			case "image":
				historyOptions.ContentType = types.TypeImage
			case "url":
				historyOptions.ContentType = types.TypeURL
			case "file":
				historyOptions.ContentType = types.TypeFile
			case "filepath":
				historyOptions.ContentType = types.TypeFilePath
			default:
				return fmt.Errorf("invalid content type: %s", itemType)
			}
		}
		
		// Log the filter options
		logger.Info("Retrieving clipboard history with filters",
			"limit", optionOrDefault(historyOptions.Limit, "none"),
			"type", optionOrDefault(string(historyOptions.ContentType), "all"),
			"reverse", historyOptions.Reverse,
			"min_size", historyOptions.MinSize)
		
		// Display the history in the requested format
		if jsonOutput {
			return displayHistoryJSON(store, historyOptions)
		}
		
		return store.LogCompleteHistory(historyOptions)
	},
}

func init() {
	// Set up flags for this command
	historyCmd.Flags().IntVar(&limit, "limit", 0, "Maximum number of history entries to retrieve (0 for all)")
	historyCmd.Flags().StringVar(&since, "since", "", "Retrieve history since this time (RFC3339 format)")
	historyCmd.Flags().StringVar(&before, "before", "", "Retrieve history before this time (RFC3339 format)")
	historyCmd.Flags().StringVar(&itemType, "type", "", "Filter by content type (text, image, url, file, filepath)")
	historyCmd.Flags().BoolVar(&reverse, "reverse", false, "Reverse history order (newest first)")
	historyCmd.Flags().IntVar(&minSize, "min-size", 0, "Minimum content size in bytes")
	historyCmd.Flags().IntVar(&contentMaxSize, "max-size", 0, "Maximum content size in bytes")
	historyCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	historyCmd.Flags().BoolVar(&dumpAll, "dump-all", false, "Dump complete history without filters")
}

// displayHistoryJSON outputs history in JSON format
func displayHistoryJSON(store *storage.BoltStorage, options config.HistoryOptions) error {
	contents, err := store.GetHistory(options)
	if err != nil {
		return fmt.Errorf("failed to get history: %v", err)
	}
	
	// Convert to a simpler structure for JSON output
	type historyItem struct {
		Type      types.ContentType `json:"type"`
		Timestamp string            `json:"timestamp"`
		Size      int               `json:"size"`
		Content   string            `json:"content"`
	}
	
	items := make([]historyItem, 0, len(contents))
	for _, content := range contents {
		preview := ""
		if len(content.Data) > 0 {
			if content.Type == types.TypeImage {
				preview = "[Binary image data]"
			} else {
				maxPreview := 100
				if len(content.Data) <= maxPreview {
					preview = string(content.Data)
				} else {
					preview = string(content.Data[:maxPreview]) + "..."
				}
			}
		}
		
		items = append(items, historyItem{
			Type:      content.Type,
			Timestamp: content.Created.Format(time.RFC3339),
			Size:      len(content.Data),
			Content:   preview,
		})
	}
	
	// Output as JSON
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(items)
}

// Helper function to display option values or defaults
func optionOrDefault(value interface{}, defaultText string) string {
	switch v := value.(type) {
	case int:
		if v == 0 {
			return defaultText
		}
		return fmt.Sprintf("%d", v)
	case string:
		if v == "" {
			return defaultText
		}
		return v
	default:
		return fmt.Sprintf("%v", value)
	}
}