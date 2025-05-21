package cmd

import (
	"fmt"
	"os"
	"github.com/berrythewa/clipman-daemon/internal/ipc"
	"github.com/spf13/cobra"
)

// flushCmd represents the flush command
var flushCmd = &cobra.Command{
	Use:   "flush",
	Short: "Flush the clipboard cache via the daemon",
	Long: `Force a flush of the clipboard cache to free up space.
This will keep the most recent items based on the keep-items setting.

The command will display the clipboard history before and after the flush
operation, unless the --quiet flag is used.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Build the flush request
		req := &ipc.Request{
			Command: "flush",
			Args: map[string]interface{}{
				"quiet": quiet,
			},
		}

		// Send the request to the daemon
		resp, err := ipc.SendRequest("", req)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: failed to contact daemon: %v\n", err)
			return err
		}

		// Handle the response
		if resp.Status == "ok" {
			if resp.Message != "" {
				fmt.Println(resp.Message)
			}
			if resp.Data != nil && !quiet {
				fmt.Printf("Flush details: %v\n", resp.Data)
			}
			return nil
		} else {
			fmt.Fprintf(os.Stderr, "Error: %s\n", resp.Message)
			return fmt.Errorf(resp.Message)
		}
	},
}

var quiet bool

func init() {
	flushCmd.Flags().BoolVar(&quiet, "quiet", false, "Don't display history before and after flush")
}