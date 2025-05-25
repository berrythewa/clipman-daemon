package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/berrythewa/clipman-daemon/internal/ipc"
)

// newClipCmd creates the clip command
func newClipCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "clip",
		Short: "Clipboard operations",
		Long: `Perform clipboard operations:
  • Get current clipboard content
  • Set clipboard content
  • Watch for clipboard changes
  • Flush clipboard history`,
	}

	// Add subcommands
	cmd.AddCommand(newClipGetCmd())
	cmd.AddCommand(newClipSetCmd())
	cmd.AddCommand(newClipWatchCmd())
	cmd.AddCommand(newClipFlushCmd())

	return cmd
}

func newClipGetCmd() *cobra.Command {
	var raw bool

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get current clipboard content",
		RunE: func(cmd *cobra.Command, args []string) error {
			resp, err := ipc.SendRequest("", &ipc.Request{
				Command: "clip.get",
			})
			if err != nil {
				return fmt.Errorf("failed to get clipboard content: %w", err)
			}

			if resp.Status != "ok" {
				return fmt.Errorf("failed to get clipboard content: %s", resp.Message)
			}

			content, ok := resp.Data.(*types.ClipboardContent)
			if !ok {
				return fmt.Errorf("invalid response data type")
			}

			if raw {
				os.Stdout.Write(content.Data)
				return nil
			}

			if useJSON {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(content)
			}

			fmt.Printf("Type: %s\n", content.Type)
			fmt.Printf("Size: %d bytes\n", len(content.Data))
			fmt.Printf("Content:\n%s\n", string(content.Data))
			return nil
		},
	}

	cmd.Flags().BoolVar(&raw, "raw", false, "output raw content without metadata")
	return cmd
}

func newClipSetCmd() *cobra.Command {
	var contentType string

	cmd := &cobra.Command{
		Use:   "set [content]",
		Short: "Set clipboard content",
		RunE: func(cmd *cobra.Command, args []string) error {
			var data []byte
			var err error

			if len(args) > 0 {
				// Content provided as argument
				data = []byte(strings.Join(args, " "))
			} else {
				// Read from stdin
				data, err = io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("failed to read from stdin: %w", err)
				}
			}

			content := &types.ClipboardContent{
				Type:    types.ContentType(contentType),
				Data:    data,
				Created: time.Now(),
			}

			resp, err := ipc.SendRequest("", &ipc.Request{
				Command: "clip.set",
				Args: map[string]interface{}{
					"content": content,
				},
			})
			if err != nil {
				return fmt.Errorf("failed to set clipboard content: %w", err)
			}

			if resp.Status != "ok" {
				return fmt.Errorf("failed to set clipboard content: %s", resp.Message)
			}

			fmt.Println("Clipboard content set successfully")
			return nil
		},
	}

	cmd.Flags().StringVarP(&contentType, "type", "t", string(types.TypeText), "content type (text, image, file, url, etc)")
	return cmd
}

func newClipWatchCmd() *cobra.Command {
	var timeout time.Duration

	cmd := &cobra.Command{
		Use:   "watch",
		Short: "Watch clipboard changes",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Info("Starting clipboard watch")

			resp, err := ipc.SendRequest("", &ipc.Request{
				Command: "clip.watch",
				Args: map[string]interface{}{
					"timeout": timeout,
				},
			})
			if err != nil {
				return fmt.Errorf("failed to watch clipboard: %w", err)
			}

			if resp.Status != "ok" {
				return fmt.Errorf("failed to watch clipboard: %s", resp.Message)
			}

			changes := make(chan *types.ClipboardContent)
			go func() {
				for content := range changes {
					if useJSON {
						enc := json.NewEncoder(os.Stdout)
						enc.SetIndent("", "  ")
						enc.Encode(content)
					} else {
						fmt.Printf("\nNew clipboard content:\n")
						fmt.Printf("Type: %s\n", content.Type)
						fmt.Printf("Size: %d bytes\n", len(content.Data))
						fmt.Printf("Content:\n%s\n", string(content.Data))
					}
				}
			}()

			// Block until interrupted
			<-cmd.Context().Done()
			return nil
		},
	}

	cmd.Flags().DurationVarP(&timeout, "timeout", "t", 0, "watch timeout duration (0 for infinite)")
	return cmd
}

func newClipFlushCmd() *cobra.Command {
	var keepLast int

	cmd := &cobra.Command{
		Use:   "flush",
		Short: "Flush clipboard history",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Info("Flushing clipboard history", zap.Int("keep_last", keepLast))

			resp, err := ipc.SendRequest("", &ipc.Request{
				Command: "clip.flush",
				Args: map[string]interface{}{
					"keep_last": keepLast,
				},
			})
			if err != nil {
				return fmt.Errorf("failed to flush clipboard history: %w", err)
			}

			if resp.Status != "ok" {
				return fmt.Errorf("failed to flush clipboard history: %s", resp.Message)
			}

			fmt.Println("Clipboard history flushed successfully")
			return nil
		},
	}

	cmd.Flags().IntVarP(&keepLast, "keep", "k", 0, "number of recent items to keep")
	return cmd
} 