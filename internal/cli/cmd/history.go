package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/storage"
	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	// History filtering flags
	limit      		int64
	since      		string
	before     		string
	itemType   		string
	reverse    		bool
	minSize    		int64
	contentMaxSize  int64
	jsonOutput 		bool
	dumpAll    		bool
	mostRecent      bool
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

  # Show the most recent item
  clipmand history --most-recent

  # Show all text items in reverse order (newest first)
  clipmand history --type text --reverse

  # Show items from a specific time range
  clipmand history --since 2023-01-01T00:00:00Z --before 2023-01-31T23:59:59Z

  # Show items larger than a specific size
  clipmand history --min-size 1024
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Get all system paths
		paths := GetConfig().GetPaths()
		
		// Check if the daemon is running by looking for lock files or processes
		isDaemonRunning := checkDaemonRunning(paths.DBFile)
		isDBLocked := checkDatabaseLock(paths.DBFile)
		
		if isDaemonRunning || isDBLocked {
			zapLogger.Info("Detected running daemon or database lock")
			
			fmt.Println("\n=== DATABASE ACCESS CONFLICT ===")
			fmt.Println("It appears that the clipman daemon is currently running.")
			fmt.Println("This is expected behavior: the clipboard daemon has an exclusive lock on the database.")
			fmt.Println("")
			fmt.Println("The clipboard database cannot be accessed by multiple processes simultaneously")
			fmt.Println("to maintain data integrity. This is not a bug, but a safety mechanism.")
			fmt.Println("")
			fmt.Println("Options:")
			fmt.Println("1. Stop the daemon first: killall clipman")
			fmt.Println("2. Use a direct clipboard access method instead:")
			fmt.Println("   - xclip -o (for X11)")
			fmt.Println("   - wl-paste (for Wayland)")
			fmt.Println("")
			fmt.Println("Future versions will include IPC support to allow history access")
			fmt.Println("while the daemon is running.")
			fmt.Println("=================================")
			
			return fmt.Errorf("daemon is running and has locked the database")
		}
		
		// Initialize storage
		storageConfig := storage.StorageConfig{
			DBPath:   paths.DBFile,
			DeviceID: GetConfig().DeviceID,
			Logger:   GetZapLogger(),
		}
		
		store, err := storage.NewBoltStorage(storageConfig)
		if err != nil {
			zapLogger.Error("Failed to initialize storage", zap.Error(err))
			
			// Check for timeout errors specifically
			if strings.Contains(err.Error(), "timeout") {
				zapLogger.Info("Database access timeout - daemon is likely running and has exclusive lock")
				
				fmt.Println("\n=== DATABASE ACCESS CONFLICT ===")
				fmt.Println("Timeout error accessing the clipboard database.")
				fmt.Println("This is expected behavior: the clipboard daemon has an exclusive lock on the database.")
				fmt.Println("")
				fmt.Println("The clipboard database cannot be accessed by multiple processes simultaneously")
				fmt.Println("to maintain data integrity. This is not a bug, but a safety mechanism.")
				fmt.Println("")
				fmt.Println("Options:")
				fmt.Println("1. Stop the daemon first: killall clipman")
				fmt.Println("2. Use a direct clipboard access method instead:")
				fmt.Println("   - xclip -o (for X11)")
				fmt.Println("   - wl-paste (for Wayland)")
				fmt.Println("")
				fmt.Println("Future versions will include IPC support to allow history access")
				fmt.Println("while the daemon is running.")
				fmt.Println("=================================")
			}
			
			return err
		}
		defer store.Close()
		
		// If --most-recent flag is set, override other settings to show just the most recent item
		if mostRecent {
			limit = 1
			reverse = true
		}
		
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
			zapLogger.Info("Dumping complete clipboard history")
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
			case "string":
				historyOptions.ContentType = types.TypeString
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
		zapLogger.Info("Retrieving clipboard history with filters",
			zap.Int64("limit", historyOptions.Limit),
			zap.String("type", string(historyOptions.ContentType)),
			zap.Bool("reverse", historyOptions.Reverse),
			zap.Int64("min_size", historyOptions.MinSize))
		
		// Display the history in the requested format
		if jsonOutput {
			return displayHistoryJSON(store, historyOptions)
		}
		
		// Custom display handling for most recent item
		if mostRecent {
			content, err := store.GetLatestContent()
			if err != nil {
				return fmt.Errorf("failed to get most recent content: %v", err)
			}
			
			if content == nil {
				fmt.Println("No clipboard history found.")
				return nil
			}
			
			fmt.Println("\n=== MOST RECENT CLIPBOARD ITEM ===")
			fmt.Printf("Timestamp: %s\n", content.Created.Format(time.RFC3339))
			fmt.Printf("Type: %s\n", content.Type)
			fmt.Printf("Size: %d bytes\n", len(content.Data))
			
			// Format content based on type
			fmt.Println("\nContent:")
			displayFormattedContent(content)
			
			return nil
		}
		
		return store.LogCompleteHistory(historyOptions)
	},
}

func init() {
	// Set up flags for this command
	historyCmd.Flags().Int64Var(&limit, "limit", 0, "Maximum number of history entries to retrieve (0 for all)")
	historyCmd.Flags().StringVar(&since, "since", "", "Retrieve history since this time (RFC3339 format)")
	historyCmd.Flags().StringVar(&before, "before", "", "Retrieve history before this time (RFC3339 format)")
	historyCmd.Flags().StringVar(&itemType, "type", "", "Filter by content type (text, image, url, file, filepath)")
	historyCmd.Flags().BoolVar(&reverse, "reverse", false, "Reverse history order (newest first)")
	historyCmd.Flags().Int64Var(&minSize, "min-size", 0, "Minimum content size in bytes")
	historyCmd.Flags().Int64Var(&contentMaxSize, "max-size", 0, "Maximum content size in bytes")
	historyCmd.Flags().BoolVar(&jsonOutput, "json", false, "Output in JSON format")
	historyCmd.Flags().BoolVar(&dumpAll, "dump-all", false, "Dump complete history without filters")
	historyCmd.Flags().BoolVar(&mostRecent, "most-recent", false, "Show only the most recent clipboard item with full details")
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
		Size      int64               `json:"size"`
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
			Size:      int64(len(content.Data)),
			Content:   preview,
		})
	}
	
	// Output as JSON
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(items)
}

// displayFormattedContent formats and displays the content in a user-friendly way
func displayFormattedContent(content *types.ClipboardContent) {
	if content == nil || len(content.Data) == 0 {
		fmt.Println("[Empty content]")
		return
	}

	switch content.Type {
	case types.TypeImage:
		fmt.Println("[Binary image data]")
		fmt.Printf("Size: %d bytes\n", len(content.Data))
	
	case types.TypeFile:
		// For file lists, try to parse JSON and display nicely
		var files []string
		if err := json.Unmarshal(content.Data, &files); err == nil && len(files) > 0 {
			fmt.Printf("File list (%d files):\n", len(files))
			for i, file := range files {
				if i < 10 || len(files) <= 15 { // Show all if 15 or fewer, otherwise first 10
					fmt.Printf("  - %s\n", file)
				} else if i == 10 {
					fmt.Printf("  - ... and %d more files\n", len(files)-10)
					break
				}
			}
		} else {
			// Display raw if not able to parse as JSON
			fmt.Println(formatMultilineContent(string(content.Data), 80))
		}
	
	case types.TypeURL:
		url := strings.TrimSpace(string(content.Data))
		fmt.Printf("URL: %s\n", url)
	
	case types.TypeFilePath:
		path := strings.TrimSpace(string(content.Data))
		fmt.Printf("File path: %s\n", path)
		
		// Check if file exists and show basic info
		if info, err := os.Stat(path); err == nil {
			fmt.Printf("  - Size: %d bytes\n", info.Size())
			fmt.Printf("  - Modified: %s\n", info.ModTime().Format(time.RFC3339))
			fmt.Printf("  - Is directory: %t\n", info.IsDir())
		} else {
			fmt.Printf("  - File does not exist or is inaccessible\n")
		}
	
	default: // Text content
		fmt.Println(formatMultilineContent(string(content.Data), 100))
	}
}

// formatMultilineContent formats text content for display, handling newlines and truncation
func formatMultilineContent(text string, maxLineLength int) string {
	lines := strings.Split(text, "\n")
	
	var result strings.Builder
	lineCount := len(lines)
	visibleLines := lineCount
	
	// Limit the number of displayed lines for very large content
	const maxVisibleLines = 25
	if lineCount > maxVisibleLines {
		visibleLines = maxVisibleLines
	}
	
	for i := 0; i < visibleLines; i++ {
		line := lines[i]
		if len(line) > maxLineLength {
			line = line[:maxLineLength] + "..."
		}
		result.WriteString(line)
		result.WriteString("\n")
	}
	
	if lineCount > visibleLines {
		result.WriteString(fmt.Sprintf("\n... [%d more lines not shown] ...\n", lineCount-visibleLines))
	}
	
	return result.String()
}

// Helper function to display option values or defaults
func optionOrDefault(value interface{}, defaultText string) string {
	switch v := value.(type) {
	case int64:
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

// checkDaemonRunning checks if the daemon is currently running by looking for
// lock files or active processes
func checkDaemonRunning(dbPath string) bool {
	// Check if the database file has a .lock file
	if _, err := os.Stat(dbPath + ".lock"); err == nil {
		return true
	}
	
	// Check for "clipman run" or "clipmand run" processes 
	cmds := []struct {
		cmd  string
		args []string
	}{
		{cmd: "pgrep", args: []string{"-f", "clipman run"}},
		{cmd: "pgrep", args: []string{"-f", "clipmand run"}},
		{cmd: "pgrep", args: []string{"-f", "clipman-daemon run"}},
	}
	
	for _, c := range cmds {
		cmd := exec.Command(c.cmd, c.args...)
		if err := cmd.Run(); err == nil {
			return true
		}
	}
	
	// Check with more specific process attributes
	processPatterns := []string{
		"clipman run", 
		"clipmand run",
		"clipman-daemon run",
	}
	
	psCmd := exec.Command("ps", "aux")
	output, err := psCmd.Output()
	if err == nil {
		outputStr := string(output)
		for _, pattern := range processPatterns {
			if strings.Contains(outputStr, pattern) {
				return true
			}
		}
	}
	
	return false
}

// checkDatabaseLock tries to open the database file in a non-blocking way to see if it's locked
func checkDatabaseLock(dbPath string) bool {
	// Open the file with O_RDONLY and no blocking
	file, err := os.OpenFile(dbPath, os.O_RDONLY, 0)
	if err != nil {
		// If we can't open the file at all, it's probably due to permissions or doesn't exist
		return false
	}
	defer file.Close()
	
	// Try to get an exclusive lock but don't block
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		// If we can't get a lock, the file is probably locked by another process
		return true
	}
	
	// Release the lock before returning
	syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	return false
}