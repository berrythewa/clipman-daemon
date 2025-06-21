package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/berrythewa/clipman-daemon/internal/ipc"
	"github.com/berrythewa/clipman-daemon/pkg/format"
)

var (
	limit     int
	offset    int
	loadMore  bool
)

// historyCmd creates the history command with all subcommands
func historyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Manage clipboard history",
		Long: `Manage clipboard history:
  • List clipboard history entries
  • Show specific history entries  
  • Delete history entries
  • Show history statistics`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default behavior: list recent history
			return executeHistoryList(format.DefaultOptions(), 10, false, "", 0, 0, 0, 0)
		},
	}

	// Add global flags for the history command
	cmd.Flags().IntVarP(&limit, "limit", "n", 10, "Number of items to display")
	cmd.Flags().BoolVar(&useJSON, "json", false, "Output history as JSON")

	// Add subcommands
	cmd.AddCommand(historyListCmd())
	cmd.AddCommand(historyShowCmd())
	cmd.AddCommand(historyDeleteCmd())
	cmd.AddCommand(historyStatsCmd())

	return cmd
}

// historyListCmd creates the list subcommand
func historyListCmd() *cobra.Command {
	var (
		limit      int
		since      time.Duration
		before     time.Duration
		reverse    bool
		typeFilter string
		minSize    int64
		maxSize    int64
		compact    bool
		noColors   bool
		noIcons    bool
		maxLines   int
		maxWidth   int
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List clipboard history",
		Long: `List clipboard history entries with various filtering and formatting options.

Examples:
  clipman history list                    # Show last 10 entries
  clipman history list -n 20              # Show last 20 entries  
  clipman history list --since 1h         # Show entries from last hour
  clipman history list --type text        # Show only text entries
  clipman history list --compact          # Compact single-line format`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Build formatting options
			opts := format.DefaultOptions()
			if compact {
				opts = format.CompactOptions()
			}
			if noColors {
				opts.UseColors = false
			}
			if noIcons {
				opts.UseIcons = false
			}
			if maxLines > 0 {
				opts.MaxLines = maxLines
			}
			if maxWidth > 0 {
				opts.MaxWidth = maxWidth
			}

			return executeHistoryList(opts, limit, reverse, typeFilter, since, before, minSize, maxSize)
		},
	}

	// Filtering flags
	cmd.Flags().IntVarP(&limit, "limit", "n", 10, "maximum number of entries to show")
	cmd.Flags().DurationVar(&since, "since", 0, "show entries since duration (e.g. 24h)")
	cmd.Flags().DurationVar(&before, "before", 0, "show entries before duration")
	cmd.Flags().BoolVarP(&reverse, "reverse", "r", false, "reverse order (newest first)")
	cmd.Flags().StringVarP(&typeFilter, "type", "t", "", "filter by content type (text, image, file, url, html)")
	cmd.Flags().Int64Var(&minSize, "min-size", 0, "minimum content size in bytes")
	cmd.Flags().Int64Var(&maxSize, "max-size", 0, "maximum content size in bytes")
	
	// Formatting flags
	cmd.Flags().BoolVarP(&compact, "compact", "c", false, "use compact single-line format")
	cmd.Flags().BoolVar(&noColors, "no-colors", false, "disable colored output")
	cmd.Flags().BoolVar(&noIcons, "no-icons", false, "disable icons in output")
	cmd.Flags().IntVar(&maxLines, "max-lines", 10, "maximum lines to show per entry (0 = no limit)")
	cmd.Flags().IntVar(&maxWidth, "max-width", 80, "maximum width per line (0 = no limit)")

	return cmd
}

// historyShowCmd creates the show subcommand
func historyShowCmd() *cobra.Command {
	var (
		raw      bool
		noColors bool
		noIcons  bool
	)

	cmd := &cobra.Command{
		Use:   "show <hash>",
		Short: "Show specific history entry",
		Long: `Show a specific history entry by its hash.

Examples:
  clipman history show abc123def        # Show entry with hash abc123def
  clipman history show abc123def --raw  # Show raw content only`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			hash := args[0]

			entry, err := getHistoryEntry(hash)
			if err != nil {
				return err
			}

			if raw {
				os.Stdout.Write(entry.Data)
				return nil
			}

			if useJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(entry)
			}

			// Build formatting options
			opts := format.DefaultOptions()
			if noColors {
				opts.UseColors = false
			}
			if noIcons {
				opts.UseIcons = false
			}
			opts.MaxLines = 0 // No line limit for single entry view
			opts.MaxWidth = 0 // No width limit for single entry view

			formatter := format.New(opts)
			output := formatter.FormatContent(entry)
			fmt.Println(output)

			return nil
		},
	}

	cmd.Flags().BoolVar(&raw, "raw", false, "output raw content without metadata")
	cmd.Flags().BoolVar(&noColors, "no-colors", false, "disable colored output")
	cmd.Flags().BoolVar(&noIcons, "no-icons", false, "disable icons in output")

	return cmd
}

// historyDeleteCmd creates the delete subcommand
func historyDeleteCmd() *cobra.Command {
	var (
		all        bool
		older      time.Duration
		typeFilter string
		force      bool
	)

	cmd := &cobra.Command{
		Use:   "delete [hash...]",
		Short: "Delete history entries",
		Long: `Delete history entries by hash or using filters.

Examples:
  clipman history delete abc123def       # Delete specific entry
  clipman history delete --all           # Delete all history
  clipman history delete --older 7d      # Delete entries older than 7 days
  clipman history delete --type image    # Delete all image entries`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger, err := GetLogger()
			if err != nil {
				return fmt.Errorf("failed to get logger: %w", err)
			}

			if !all && older == 0 && typeFilter == "" && len(args) == 0 {
				return fmt.Errorf("specify entries to delete by hash, or use --all/--older/--type flags")
			}

			if !force && (all || older > 0 || typeFilter != "") {
				fmt.Print("This will permanently delete clipboard history entries. Continue? (y/N): ")
				var response string
				fmt.Scanln(&response)
				if response != "y" && response != "Y" && response != "yes" {
					logger.Info("History deletion cancelled by user")
					fmt.Println("Deletion cancelled.")
					return nil
				}
			}

			logger.Info("Deleting history entries",
				zap.Bool("all", all),
				zap.Duration("older", older),
				zap.String("type_filter", typeFilter),
				zap.Strings("hashes", args),
				zap.Bool("force", force))

			count, err := deleteHistoryEntries(args, all, older, typeFilter)
			if err != nil {
				logger.Error("Failed to delete history entries", zap.Error(err))
				return err
			}

			logger.Info("Successfully deleted history entries", zap.Int("count", count))
			fmt.Printf("✓ Deleted %d entries\n", count)
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "delete all history")
	cmd.Flags().DurationVar(&older, "older", 0, "delete entries older than duration (e.g. 7d, 24h)")
	cmd.Flags().StringVarP(&typeFilter, "type", "t", "", "delete entries of specific type")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "skip confirmation prompt")

	return cmd
}

// historyStatsCmd creates the stats subcommand
func historyStatsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show history statistics",
		Long: `Show statistics about clipboard history.

Examples:
  clipman history stats                   # Show basic statistics
  clipman history stats --json           # Show statistics as JSON`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger, err := GetLogger()
			if err != nil {
				return fmt.Errorf("failed to get logger: %w", err)
			}

			logger.Info("Retrieving history statistics")

			stats, err := getHistoryStats()
			if err != nil {
				logger.Error("Failed to get history statistics", zap.Error(err))
				return err
			}

			if useJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(stats)
			}

			// Format statistics for display
			fmt.Printf("📊 Clipboard History Statistics\n\n")
			fmt.Printf("Total entries: %v\n", stats["total_entries"])
			fmt.Printf("Total size: %v bytes\n", stats["total_size"])
			
			if typeCounts, ok := stats["type_counts"].(map[string]interface{}); ok {
				fmt.Printf("\nBy type:\n")
				for contentType, count := range typeCounts {
					fmt.Printf("  %s: %v\n", contentType, count)
				}
			}

			if oldest, ok := stats["oldest_entry"].(map[string]interface{}); ok {
				fmt.Printf("\nOldest entry:\n")
				fmt.Printf("  Hash: %v\n", oldest["hash"])
				fmt.Printf("  Type: %v\n", oldest["type"])
				fmt.Printf("  Created: %v\n", oldest["created"])
				fmt.Printf("  Size: %v bytes\n", oldest["size"])
			}

			if newest, ok := stats["newest_entry"].(map[string]interface{}); ok {
				fmt.Printf("\nNewest entry:\n")
				fmt.Printf("  Hash: %v\n", newest["hash"])
				fmt.Printf("  Type: %v\n", newest["type"])
				fmt.Printf("  Created: %v\n", newest["created"])
				fmt.Printf("  Size: %v bytes\n", newest["size"])
			}

			logger.Info("History statistics displayed successfully")
			return nil
		},
	}

	return cmd
}

// executeHistoryList handles the history list functionality
func executeHistoryList(opts format.Options, limit int, reverse bool, typeFilter string, since, before time.Duration, minSize, maxSize int64) error {
	logger, err := GetLogger()
	if err != nil {
		return fmt.Errorf("failed to get logger: %w", err)
	}

	now := time.Now()
	req := &ipc.Request{
		Command: "history.list",
		Args: map[string]interface{}{
			"limit":   limit,
			"reverse": reverse,
		},
	}

	// Add time filters
	if since > 0 {
		req.Args["since"] = now.Add(-since).Format(time.RFC3339)
	}
	if before > 0 {
		req.Args["before"] = now.Add(-before).Format(time.RFC3339)
	}
	
	// Add content filters
	if typeFilter != "" {
		req.Args["type"] = typeFilter
	}
	if minSize > 0 {
		req.Args["min_size"] = minSize
	}
	if maxSize > 0 {
		req.Args["max_size"] = maxSize
	}

	logger.Info("Requesting history list",
		zap.Int("limit", limit),
		zap.Bool("reverse", reverse),
		zap.String("type_filter", typeFilter),
		zap.Duration("since", since),
		zap.Duration("before", before),
		zap.Int64("min_size", minSize),
		zap.Int64("max_size", maxSize))

	// Send IPC request
	resp, err := ipc.SendRequest(ipc.DefaultSocketPath, req)
	if err != nil {
		logger.Error("Failed to connect to daemon", zap.Error(err))
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}

	if resp.Status != "ok" {
		logger.Error("Daemon returned error", zap.String("status", resp.Status), zap.String("message", resp.Message))
		return fmt.Errorf("daemon error: %s", resp.Message)
	}

	// Parse response data
	entries, err := parseClipboardContentList(resp.Data)
	if err != nil {
		logger.Error("Failed to parse response", zap.Error(err))
		return fmt.Errorf("failed to parse response: %w", err)
	}

	logger.Info("Successfully retrieved history entries", zap.Int("count", len(entries)))

	// Handle JSON output
	if useJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}

	// Format and display using the format package
	formatter := format.New(opts)
	output := formatter.FormatContentList(entries)
	fmt.Println(output)

	return nil
}

// getHistoryEntry retrieves a specific history entry by hash via IPC
func getHistoryEntry(hash string) (*types.ClipboardContent, error) {
	logger, err := GetLogger()
	if err != nil {
		return nil, fmt.Errorf("failed to get logger: %w", err)
	}

	logger.Info("Retrieving history entry", zap.String("hash", hash))

	resp, err := ipc.SendRequest(ipc.DefaultSocketPath, &ipc.Request{
		Command: "history.show",
		Args: map[string]interface{}{
			"hash": hash,
		},
	})
	if err != nil {
		logger.Error("Failed to connect to daemon", zap.Error(err))
		return nil, fmt.Errorf("failed to connect to daemon: %w", err)
	}

	if resp.Status != "ok" {
		logger.Error("Daemon returned error", zap.String("status", resp.Status), zap.String("message", resp.Message))
		return nil, fmt.Errorf("daemon error: %s", resp.Message)
	}

	// Parse single entry
	entry, err := parseClipboardContent(resp.Data)
	if err != nil {
		logger.Error("Failed to parse entry", zap.Error(err))
		return nil, fmt.Errorf("failed to parse entry: %w", err)
	}

	logger.Info("Successfully retrieved history entry", 
		zap.String("hash", hash),
		zap.String("type", string(entry.Type)),
		zap.Int("size", len(entry.Data)))

	return entry, nil
}

// deleteHistoryEntries deletes history entries via IPC
func deleteHistoryEntries(hashes []string, all bool, older time.Duration, typeFilter string) (int, error) {
	logger, err := GetLogger()
	if err != nil {
		return 0, fmt.Errorf("failed to get logger: %w", err)
	}

	req := &ipc.Request{
		Command: "history.delete",
		Args:    make(map[string]interface{}),
	}

	if all {
		req.Args["all"] = true
	}
	if older > 0 {
		req.Args["older_than"] = time.Now().Add(-older).Format(time.RFC3339)
	}
	if typeFilter != "" {
		req.Args["type"] = typeFilter
	}
	if len(hashes) > 0 {
		req.Args["hashes"] = hashes
	}

	logger.Info("Sending delete request to daemon",
		zap.Bool("all", all),
		zap.Duration("older", older),
		zap.String("type_filter", typeFilter),
		zap.Strings("hashes", hashes))

	resp, err := ipc.SendRequest(ipc.DefaultSocketPath, req)
	if err != nil {
		logger.Error("Failed to connect to daemon", zap.Error(err))
		return 0, fmt.Errorf("failed to connect to daemon: %w", err)
	}

	if resp.Status != "ok" {
		logger.Error("Daemon returned error", zap.String("status", resp.Status), zap.String("message", resp.Message))
		return 0, fmt.Errorf("daemon error: %s", resp.Message)
	}

	count, ok := resp.Data.(float64) // JSON unmarshaling converts int to float64
	if !ok {
		logger.Error("Invalid response data type", zap.Any("data", resp.Data))
		return 0, fmt.Errorf("invalid response data type: %T", resp.Data)
	}

	logger.Info("Successfully deleted history entries", zap.Int("count", int(count)))
	return int(count), nil
}

// getHistoryStats retrieves history statistics via IPC
func getHistoryStats() (map[string]interface{}, error) {
	logger, err := GetLogger()
	if err != nil {
		return nil, fmt.Errorf("failed to get logger: %w", err)
	}

	logger.Info("Requesting history statistics")

	resp, err := ipc.SendRequest(ipc.DefaultSocketPath, &ipc.Request{
		Command: "history.stats",
	})
	if err != nil {
		logger.Error("Failed to connect to daemon", zap.Error(err))
		return nil, fmt.Errorf("failed to connect to daemon: %w", err)
	}

	if resp.Status != "ok" {
		logger.Error("Daemon returned error", zap.String("status", resp.Status), zap.String("message", resp.Message))
		return nil, fmt.Errorf("daemon error: %s", resp.Message)
	}

	stats, ok := resp.Data.(map[string]interface{})
	if !ok {
		logger.Error("Invalid response data type", zap.Any("data", resp.Data))
		return nil, fmt.Errorf("invalid response data type: %T", resp.Data)
	}

	logger.Info("Successfully retrieved history statistics")
	return stats, nil
}

// parseClipboardContentList parses a list of clipboard content from IPC response
func parseClipboardContentList(data interface{}) ([]*types.ClipboardContent, error) {
	logger, err := GetLogger()
	if err != nil {
		return nil, fmt.Errorf("failed to get logger: %w", err)
	}

	// Handle the case where data is already a slice
	if entries, ok := data.([]*types.ClipboardContent); ok {
		logger.Debug("Data is already ClipboardContent slice", zap.Int("count", len(entries)))
		return entries, nil
	}

	// Handle JSON unmarshaling
	jsonData, err := json.Marshal(data)
	if err != nil {
		logger.Error("Failed to marshal data", zap.Error(err))
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	var entries []*types.ClipboardContent
	if err := json.Unmarshal(jsonData, &entries); err != nil {
		logger.Error("Failed to unmarshal entries", zap.Error(err))
		return nil, fmt.Errorf("failed to unmarshal entries: %w", err)
	}

	logger.Debug("Successfully parsed clipboard content list", zap.Int("count", len(entries)))
	return entries, nil
}

// parseClipboardContent parses a single clipboard content from IPC response
func parseClipboardContent(data interface{}) (*types.ClipboardContent, error) {
	logger, err := GetLogger()
	if err != nil {
		return nil, fmt.Errorf("failed to get logger: %w", err)
	}

	// Handle the case where data is already a ClipboardContent
	if content, ok := data.(*types.ClipboardContent); ok {
		logger.Debug("Data is already ClipboardContent")
		return content, nil
	}

	// Handle JSON unmarshaling
	jsonData, err := json.Marshal(data)
	if err != nil {
		logger.Error("Failed to marshal data", zap.Error(err))
		return nil, fmt.Errorf("failed to marshal data: %w", err)
	}

	var content types.ClipboardContent
	if err := json.Unmarshal(jsonData, &content); err != nil {
		logger.Error("Failed to unmarshal content", zap.Error(err))
		return nil, fmt.Errorf("failed to unmarshal content: %w", err)
	}

	logger.Debug("Successfully parsed clipboard content")
	return &content, nil
}