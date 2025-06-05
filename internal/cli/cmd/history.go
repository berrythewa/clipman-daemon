package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/berrythewa/clipman-daemon/internal/ipc"
	"github.com/berrythewa/clipman-daemon/pkg/format"
)

var (
	limit     int
	offset    int
	loadMore  bool
)

// historyCmd represents the history command
var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Manage clipboard history",
	Long: `Manage clipboard history:
  • List clipboard history entries
  • Show specific history entries
  • Delete history entries
  • Show history statistics`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Default behavior: list recent history
		return executeHistoryList(format.DefaultOptions(), limit, false, "", 0, 0, 0, 0)
	},
}

func init() {
	historyCmd.Flags().IntVarP(&limit, "limit", "n", 10, "Number of items to display")
	historyCmd.Flags().IntVar(&offset, "offset", 0, "Offset for pagination (start at this item)")
	historyCmd.Flags().BoolVar(&useJSON, "json", false, "Output history as JSON")
	historyCmd.Flags().BoolVar(&loadMore, "load-more", false, "Load more items from DB after cache is exhausted")
}

// newHistoryCmd creates the history command with all subcommands
func newHistoryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "history",
		Short: "Manage clipboard history",
		Long: `Manage clipboard history:
  • List clipboard history entries
  • Show specific history entries  
  • Delete history entries
  • Show history statistics`,
	}

	// Add subcommands
	cmd.AddCommand(newHistoryListCmd())
	cmd.AddCommand(newHistoryShowCmd())
	cmd.AddCommand(newHistoryDeleteCmd())
	cmd.AddCommand(newHistoryStatsCmd())

	return cmd
}

// newHistoryListCmd creates the list subcommand
func newHistoryListCmd() *cobra.Command {
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

// newHistoryShowCmd creates the show subcommand
func newHistoryShowCmd() *cobra.Command {
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

// newHistoryDeleteCmd creates the delete subcommand
func newHistoryDeleteCmd() *cobra.Command {
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
			if !all && older == 0 && typeFilter == "" && len(args) == 0 {
				return fmt.Errorf("specify entries to delete by hash, or use --all/--older/--type flags")
			}

			if !force && (all || older > 0 || typeFilter != "") {
				fmt.Print("This will permanently delete clipboard history entries. Continue? (y/N): ")
				var response string
				fmt.Scanln(&response)
				if response != "y" && response != "Y" && response != "yes" {
					fmt.Println("Deletion cancelled.")
					return nil
				}
			}

			count, err := deleteHistoryEntries(args, all, older, typeFilter)
			if err != nil {
				return err
			}

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

// newHistoryStatsCmd creates the stats subcommand
func newHistoryStatsCmd() *cobra.Command {
	var (
		noColors bool
		noIcons  bool
	)

	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Show history statistics",
		Long: `Show comprehensive statistics about clipboard history.

Examples:
  clipman history stats           # Show detailed statistics
  clipman history stats --json   # Output statistics as JSON`,
		RunE: func(cmd *cobra.Command, args []string) error {
			stats, err := getHistoryStats()
			if err != nil {
				return err
			}

			if useJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(stats)
			}

			// Build formatting options
			opts := format.DefaultOptions()
			if noColors {
				opts.UseColors = false
			}
			if noIcons {
				opts.UseIcons = false
			}

			formatter := format.New(opts)
			output := formatter.FormatStats(stats)
			fmt.Println(output)

			return nil
		},
	}

	cmd.Flags().BoolVar(&noColors, "no-colors", false, "disable colored output")
	cmd.Flags().BoolVar(&noIcons, "no-icons", false, "disable icons in output")

	return cmd
}

// Helper functions for IPC communication

// executeHistoryList executes the history list command via IPC
func executeHistoryList(opts format.Options, limit int, reverse bool, typeFilter string, since, before time.Duration, minSize, maxSize int64) error {
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

	// Send IPC request
	resp, err := ipc.SendRequest(ipc.DefaultSocketPath, req)
	if err != nil {
		return fmt.Errorf("failed to connect to daemon: %w", err)
	}

	if resp.Status != "ok" {
		return fmt.Errorf("daemon error: %s", resp.Message)
	}

	// Parse response data
	entries, err := parseClipboardContentList(resp.Data)
	if err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}

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
	resp, err := ipc.SendRequest(ipc.DefaultSocketPath, &ipc.Request{
		Command: "history.show",
		Args: map[string]interface{}{
			"hash": hash,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to daemon: %w", err)
	}

	if resp.Status != "ok" {
		return nil, fmt.Errorf("daemon error: %s", resp.Message)
	}

	// Parse single entry
	entry, err := parseClipboardContent(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse entry: %w", err)
	}

	return entry, nil
}

// deleteHistoryEntries deletes history entries via IPC
func deleteHistoryEntries(hashes []string, all bool, older time.Duration, typeFilter string) (int, error) {
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

	resp, err := ipc.SendRequest(ipc.DefaultSocketPath, req)
	if err != nil {
		return 0, fmt.Errorf("failed to connect to daemon: %w", err)
	}

	if resp.Status != "ok" {
		return 0, fmt.Errorf("daemon error: %s", resp.Message)
	}

	count, ok := resp.Data.(float64) // JSON unmarshaling converts int to float64
	if !ok {
		return 0, fmt.Errorf("invalid response data type: %T", resp.Data)
	}

	return int(count), nil
}

// getHistoryStats retrieves history statistics via IPC
func getHistoryStats() (map[string]interface{}, error) {
	resp, err := ipc.SendRequest(ipc.DefaultSocketPath, &ipc.Request{
		Command: "history.stats",
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to daemon: %w", err)
	}

	if resp.Status != "ok" {
		return nil, fmt.Errorf("daemon error: %s", resp.Message)
	}

	stats, ok := resp.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid response data type")
	}

	return stats, nil
}

// Utility functions for parsing IPC response data

// parseClipboardContentList converts IPC response data to ClipboardContent list
func parseClipboardContentList(data interface{}) ([]*types.ClipboardContent, error) {
	dataSlice, ok := data.([]interface{})
	if !ok {
		return nil, fmt.Errorf("expected array, got %T", data)
	}

	var entries []*types.ClipboardContent
	for _, item := range dataSlice {
		entry, err := parseClipboardContent(item)
		if err != nil {
			return nil, fmt.Errorf("failed to parse entry: %w", err)
		}
		entries = append(entries, entry)
	}

	return entries, nil
}

// parseClipboardContent converts IPC response data to ClipboardContent
func parseClipboardContent(data interface{}) (*types.ClipboardContent, error) {
	itemMap, ok := data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("expected object, got %T", data)
	}

	entry := &types.ClipboardContent{}

	// Parse type
	if typeStr, ok := itemMap["type"].(string); ok {
		entry.Type = types.ContentType(typeStr)
	}

	// Parse data
	if dataStr, ok := itemMap["data"].(string); ok {
		entry.Data = []byte(dataStr)
	}

	// Parse timestamp
	if createdStr, ok := itemMap["created"].(string); ok {
		if created, err := time.Parse(time.RFC3339, createdStr); err == nil {
			entry.Created = created
		}
	}

	// Parse hash
	if hashStr, ok := itemMap["hash"].(string); ok {
		entry.Hash = hashStr
	}

	// Parse occurrences if present
	if occurrences, ok := itemMap["occurrences"].([]interface{}); ok {
		for _, occ := range occurrences {
			if occStr, ok := occ.(string); ok {
				if occTime, err := time.Parse(time.RFC3339, occStr); err == nil {
					entry.Occurrences = append(entry.Occurrences, occTime)
				}
			}
		}
	}

	return entry, nil
}