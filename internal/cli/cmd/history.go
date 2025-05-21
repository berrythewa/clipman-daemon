package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"github.com/berrythewa/clipman-daemon/internal/ipc"
	"github.com/spf13/cobra"
)

var (
	limit     int
	offset    int
	jsonOut   bool
	loadMore  bool
)

// historyCmd represents the history command
var historyCmd = &cobra.Command{
	Use:   "history",
	Short: "Display clipboard history (cache and DB) via the daemon",
	Long: `Display clipboard history with filtering and pagination options.
Supports loading more items from the DB after the cache is exhausted.

Examples:
  clipman history --limit 10
  clipman history --load-more --limit 20 --offset 10
  clipman history --json
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Build the history request
		req := &ipc.Request{
			Command: "history",
			Args: map[string]interface{}{
				"limit":    limit,
				"offset":   offset,
				"json":     jsonOut,
				"load_more": loadMore,
			},
		}

		// Send the request to the daemon
		resp, err := ipc.SendRequest("", req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to contact daemon: %v\n", err)
			return err
		}

		if resp.Status != "ok" {
			fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Message)
			return fmt.Errorf(resp.Message)
		}

		// Print the history
		if jsonOut {
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
	historyCmd.Flags().BoolVar(&jsonOut, "json", false, "Output history as JSON")
	historyCmd.Flags().BoolVar(&loadMore, "load-more", false, "Load more items from DB after cache is exhausted")
}