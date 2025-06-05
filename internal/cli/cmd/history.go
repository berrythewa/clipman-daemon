package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/berrythewa/clipman-daemon/internal/ipc"
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
		// Build the history request
		req := &ipc.Request{
			Command: "history",
			Args: map[string]interface{}{
				"limit":     limit,
				"offset":    offset,
				"load_more": loadMore,
			},
		}

		// Send the request to the daemon
		resp, err := ipc.SendRequest(ipc.DefaultSocketPath, req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to contact daemon: %v\n", err)
			return err
		}

		if resp.Status != "ok" {
			fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Message)
			return fmt.Errorf(resp.Message)
		}

		// Print the history
		if useJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(resp.Data)
		}

		// Assume resp.Data is a slice of history items (cache + db)
		items, ok := resp.Data.([]interface{})
		if !ok {
			fmt.Println("No history data returned.")
			return nil
		}
		for i, item := range items {
			fmt.Printf("[%d] %v\n", offset+i+1, item)
		}
		return nil
	},
}

func init() {
	historyCmd.Flags().IntVarP(&limit, "limit", "n", 10, "Number of items to display")
	historyCmd.Flags().IntVar(&offset, "offset", 0, "Offset for pagination (start at this item)")
	historyCmd.Flags().BoolVar(&useJSON, "json", false, "Output history as JSON")
	historyCmd.Flags().BoolVar(&loadMore, "load-more", false, "Load more items from DB after cache is exhausted")
}

// newHistoryCmd creates the history command
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

func newHistoryListCmd() *cobra.Command {
	var (
		since      time.Duration
		before     time.Duration
		reverse    bool
		typeFilter string
		minSize    int64
		maxSize    int64
	)

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List clipboard history",
		RunE: func(cmd *cobra.Command, args []string) error {
			now := time.Now()
			req := &ipc.Request{
				Command: "history.list",
				Args: map[string]interface{}{
					"limit":   limit,
					"reverse": reverse,
				},
			}

			if since > 0 {
				req.Args["since"] = now.Add(-since)
			}
			if before > 0 {
				req.Args["before"] = now.Add(-before)
			}
			if typeFilter != "" {
				req.Args["type"] = typeFilter
			}
			if minSize > 0 {
				req.Args["min_size"] = minSize
			}
			if maxSize > 0 {
				req.Args["max_size"] = maxSize
			}

			resp, err := ipc.SendRequest(ipc.DefaultSocketPath, req)
			if err != nil {
				return fmt.Errorf("failed to get history: %w", err)
			}

			if resp.Status != "ok" {
				return fmt.Errorf("failed to get history: %s", resp.Message)
			}

			// Handle JSON unmarshaling - resp.Data comes back as []interface{}
			var entries []*types.ClipboardContent
			if dataSlice, ok := resp.Data.([]interface{}); ok {
				// Convert each interface{} to ClipboardContent
				for _, item := range dataSlice {
					if itemMap, ok := item.(map[string]interface{}); ok {
						entry := &types.ClipboardContent{}
						
						// Convert map fields to ClipboardContent fields
						if typeStr, ok := itemMap["type"].(string); ok {
							entry.Type = types.ContentType(typeStr)
						}
						if dataStr, ok := itemMap["data"].(string); ok {
							entry.Data = []byte(dataStr)
						}
						if createdStr, ok := itemMap["created"].(string); ok {
							if created, err := time.Parse(time.RFC3339, createdStr); err == nil {
								entry.Created = created
							}
						}
						if hashStr, ok := itemMap["hash"].(string); ok {
							entry.Hash = hashStr
						}
						
						entries = append(entries, entry)
					}
				}
			} else {
				return fmt.Errorf("invalid response data type - expected array, got %T", resp.Data)
			}

			if useJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(entries)
			}

			for i, entry := range entries {
				fmt.Printf("\nEntry %d:\n", i+1)
				fmt.Printf("  Type: %s\n", entry.Type)
				fmt.Printf("  Size: %d bytes\n", len(entry.Data))
				fmt.Printf("  Created: %s\n", entry.Created.Format(time.RFC3339))
				if entry.Hash != "" {
					fmt.Printf("  Hash: %s\n", entry.Hash)
				}
				fmt.Printf("  Content:\n%s\n", formatContent(entry))
			}

			return nil
		},
	}

	cmd.Flags().IntVarP(&limit, "limit", "n", 10, "maximum number of entries to show")
	cmd.Flags().DurationVar(&since, "since", 0, "show entries since duration (e.g. 24h)")
	cmd.Flags().DurationVar(&before, "before", 0, "show entries before duration")
	cmd.Flags().BoolVarP(&reverse, "reverse", "r", false, "reverse order (newest first)")
	cmd.Flags().StringVarP(&typeFilter, "type", "t", "", "filter by content type")
	cmd.Flags().Int64Var(&minSize, "min-size", 0, "minimum content size in bytes")
	cmd.Flags().Int64Var(&maxSize, "max-size", 0, "maximum content size in bytes")

	return cmd
}

func newHistoryShowCmd() *cobra.Command {
	var raw bool

	cmd := &cobra.Command{
		Use:   "show <hash>",
		Short: "Show specific history entry",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			hash := args[0]

			resp, err := ipc.SendRequest(ipc.DefaultSocketPath, &ipc.Request{
				Command: "history.show",
				Args: map[string]interface{}{
					"hash": hash,
				},
			})
			if err != nil {
				return fmt.Errorf("failed to get entry: %w", err)
			}

			if resp.Status != "ok" {
				return fmt.Errorf("failed to get entry: %s", resp.Message)
			}

			entry, ok := resp.Data.(*types.ClipboardContent)
			if !ok {
				return fmt.Errorf("invalid response data type")
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

			fmt.Printf("Type: %s\n", entry.Type)
			fmt.Printf("Size: %d bytes\n", len(entry.Data))
			fmt.Printf("Created: %s\n", entry.Created.Format(time.RFC3339))
			fmt.Printf("Hash: %s\n", entry.Hash)
			fmt.Printf("Content:\n%s\n", formatContent(entry))

			return nil
		},
	}

	cmd.Flags().BoolVar(&raw, "raw", false, "output raw content without metadata")
	return cmd
}

func newHistoryDeleteCmd() *cobra.Command {
	var (
		all        bool
		older      time.Duration
		typeFilter string
	)

	cmd := &cobra.Command{
		Use:   "delete [hash...]",
		Short: "Delete history entries",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !all && older == 0 && typeFilter == "" && len(args) == 0 {
				return fmt.Errorf("specify entries to delete by hash, or use --all/--older/--type flags")
			}

			req := &ipc.Request{
				Command: "history.delete",
				Args:    make(map[string]interface{}),
			}

			if all {
				req.Args["all"] = true
			}
			if older > 0 {
				req.Args["older_than"] = time.Now().Add(-older)
			}
			if typeFilter != "" {
				req.Args["type"] = typeFilter
			}
			if len(args) > 0 {
				req.Args["hashes"] = args
			}

			resp, err := ipc.SendRequest(ipc.DefaultSocketPath, req)
			if err != nil {
				return fmt.Errorf("failed to delete entries: %w", err)
			}

			if resp.Status != "ok" {
				return fmt.Errorf("failed to delete entries: %s", resp.Message)
			}

			count, ok := resp.Data.(int)
			if !ok {
				return fmt.Errorf("invalid response data type")
			}

			fmt.Printf("Deleted %d entries\n", count)
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "delete all history")
	cmd.Flags().DurationVar(&older, "older", 0, "delete entries older than duration")
	cmd.Flags().StringVarP(&typeFilter, "type", "t", "", "delete entries of specific type")

	return cmd
}

func newHistoryStatsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stats",
		Short: "Show history statistics",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := ipc.SendRequest(ipc.DefaultSocketPath, &ipc.Request{
				Command: "history.stats",
			})
			if err != nil {
				return fmt.Errorf("failed to get stats: %w", err)
			}

			if resp.Status != "ok" {
				return fmt.Errorf("failed to get stats: %s", resp.Message)
			}

			stats, ok := resp.Data.(map[string]interface{})
			if !ok {
				return fmt.Errorf("invalid response data type")
			}

			if useJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(stats)
			}

			fmt.Printf("Total entries: %d\n", stats["total_entries"])
			fmt.Printf("Total size: %s\n", formatSize(stats["total_size"].(int64)))
			fmt.Printf("Oldest entry: %s\n", stats["oldest_entry"].(time.Time).Format(time.RFC3339))
			fmt.Printf("Newest entry: %s\n", stats["newest_entry"].(time.Time).Format(time.RFC3339))

			fmt.Println("\nEntries by type:")
			for typ, count := range stats["entries_by_type"].(map[string]int) {
				fmt.Printf("  %s: %d\n", typ, count)
			}

			return nil
		},
	}
}

// formatContent formats clipboard content for display
func formatContent(content *types.ClipboardContent) string {
	switch content.Type {
	case types.TypeImage:
		return "[Binary image data]"
	case types.TypeFile:
		var files []string
		if err := json.Unmarshal(content.Data, &files); err == nil {
			return fmt.Sprintf("[Files: %s]", strings.Join(files, ", "))
		}
		return string(content.Data)
	default:
		return string(content.Data)
	}
}

// formatSize formats a size in bytes to a human-readable string
func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}