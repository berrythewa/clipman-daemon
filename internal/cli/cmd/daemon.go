package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/berrythewa/clipman-daemon/internal/daemon"
)

// newDaemonCmd creates the daemon command
func newDaemonCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "daemon",
		Short: "Manage the Clipman daemon",
		Long: `Manage the Clipman daemon process that handles clipboard monitoring and syncing.

The daemon can be:
  • Started in the foreground or background
  • Stopped gracefully
  • Checked for status
  • Restarted if needed`,
	}

	// Add subcommands
	cmd.AddCommand(newDaemonStartCmd())
	cmd.AddCommand(newDaemonStopCmd())
	cmd.AddCommand(newDaemonStatusCmd())
	cmd.AddCommand(newDaemonRestartCmd())

	return cmd
}

func newDaemonStartCmd() *cobra.Command {
	var background bool

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the Clipman daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if background {
				logger.Info("Starting Clipman daemon in background")
				return daemon.Start()
			}

			logger.Info("Starting Clipman daemon in foreground")
			return daemon.RunForeground()
		},
	}

	cmd.Flags().BoolVarP(&background, "background", "b", false, "run in background")
	return cmd
}

func newDaemonStopCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop the Clipman daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Info("Stopping Clipman daemon", zap.Bool("force", force))
			
			if err := daemon.Kill(); err != nil {
				if !force {
					return fmt.Errorf("failed to stop daemon: %w", err)
				}
				logger.Warn("Failed to stop daemon gracefully, forcing", zap.Error(err))
				return daemon.Kill()
			}
			
			fmt.Println("Daemon stopped successfully")
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "force stop if graceful shutdown fails")
	return cmd
}

func newDaemonStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show daemon status",
		RunE: func(cmd *cobra.Command, args []string) error {
			isRunning, err := daemon.Status()
			if err != nil {
				return fmt.Errorf("failed to get daemon status: %w", err)
			}

			if isRunning {
				fmt.Println("Clipman daemon is running.")
			} else {
				fmt.Println("Clipman daemon is not running.")
			}
			return nil
		},
	}
}

func newDaemonRestartCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "restart",
		Short: "Restart the Clipman daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger.Info("Restarting Clipman daemon")

			// Stop the daemon
			if err := daemon.Kill(); err != nil {
				if !force {
					return fmt.Errorf("failed to stop daemon: %w", err)
				}
				logger.Warn("Failed to stop daemon gracefully, forcing", zap.Error(err))
				if err := daemon.Kill(); err != nil {
					return fmt.Errorf("failed to force stop daemon: %w", err)
				}
			}

			// Wait a moment for cleanup
			time.Sleep(time.Second)

			// Start the daemon
			if err := daemon.Start(); err != nil {
				return fmt.Errorf("failed to start daemon: %w", err)
			}

			fmt.Println("Daemon restarted successfully")
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "force restart if graceful shutdown fails")
	return cmd
}

func RunForeground() error {
	return daemon.Start()
}

func Kill() error {
	return daemon.Kill()
}

func Status() (bool, error) {
	return daemon.Status()
} 