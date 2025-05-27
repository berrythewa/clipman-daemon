package cmd

import (
	"fmt"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/common"
	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/gui"
	"github.com/berrythewa/clipman-daemon/internal/ipc"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var guiCmd = &cobra.Command{
	Use:   "gui",
	Short: "Launch the Clipman GUI",
	Long: `Launch the Clipman GUI application.
The GUI provides a user-friendly interface for managing clipboard history and device synchronization.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Load configuration
		cfg, err := config.Load()
		if err != nil {
			return fmt.Errorf("failed to load config: %w", err)
		}

		// Initialize logger
		logger, err := common.NewLogger(cfg)
		if err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}
		defer logger.Sync()

		// Check if daemon is running
		req := &ipc.Request{Command: "ping"}
		_, err = ipc.SendRequest(cfg.IPC.SocketPath, req)
		if err != nil {
			logger.Info("Daemon not running, starting it...")
			// Start daemon
			daemonCmd := &cobra.Command{}
			if err := daemonCmd.RunE(daemonCmd, []string{}); err != nil {
				return fmt.Errorf("failed to start daemon: %w", err)
			}
			// Wait for daemon to start
			for i := 0; i < 5; i++ {
				_, err = ipc.SendRequest(cfg.IPC.SocketPath, req)
				if err == nil {
					break
				}
				time.Sleep(time.Second)
			}
			if err != nil {
				return fmt.Errorf("daemon failed to start: %w", err)
			}
		}

		// Create and run GUI
		app, err := gui.NewApp(cfg, logger)
		if err != nil {
			return fmt.Errorf("failed to create GUI: %w", err)
		}

		app.Run()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(guiCmd)
} 