package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

	"github.com/berrythewa/clipman-daemon/internal/ipc"
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
			logger := GetZapLogger()
			if background {
				logger.Info("Starting Clipman daemon in background")
				return startDaemonProcess(true)
			}

			logger.Info("Starting Clipman daemon in foreground")
			return startDaemonProcess(false)
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
			logger := GetZapLogger()
			logger.Info("Stopping Clipman daemon", zap.Bool("force", force))
			
			if err := stopDaemonProcess(force); err != nil {
				return fmt.Errorf("failed to stop daemon: %w", err)
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
			isRunning, pid, err := getDaemonStatus()
			if err != nil {
				return fmt.Errorf("failed to get daemon status: %w", err)
			}

			if isRunning {
				fmt.Printf("Clipman daemon is running (PID: %d)\n", pid)
			} else {
				fmt.Println("Clipman daemon is not running")
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
			logger := GetZapLogger()
			logger.Info("Restarting Clipman daemon")

			// Stop the daemon
			if err := stopDaemonProcess(force); err != nil {
				if !force {
					return fmt.Errorf("failed to stop daemon: %w", err)
				}
				logger.Warn("Failed to stop daemon gracefully, forcing", zap.Error(err))
			}

			// Wait a moment for cleanup
			time.Sleep(time.Second)

			// Start the daemon
			if err := startDaemonProcess(true); err != nil {
				return fmt.Errorf("failed to start daemon: %w", err)
			}

			fmt.Println("Daemon restarted successfully")
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "force restart if graceful shutdown fails")
	return cmd
}

// startDaemonProcess starts the daemon as a separate process
func startDaemonProcess(background bool) error {
	// Look for clipmand binary in the same directory as the current executable
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}
	
	// Construct path to clipmand binary
	execDir := filepath.Dir(executable)
	daemonBinary := filepath.Join(execDir, "clipmand")
	
	// Check if clipmand exists
	if _, err := os.Stat(daemonBinary); os.IsNotExist(err) {
		return fmt.Errorf("clipmand binary not found at %s", daemonBinary)
	}

	// Check if daemon is already running
	if isRunning, pid, _ := getDaemonStatus(); isRunning {
		return fmt.Errorf("daemon is already running (PID: %d)", pid)
	}

	// Prepare command arguments
	var args []string
	if !background {
		args = append(args, "--foreground")
	}

	if background {
		// Start daemon in background
		cmd := exec.Command(daemonBinary, args...)
		cmd.Stdout = nil
		cmd.Stderr = nil
		cmd.Stdin = nil
		
		if err := cmd.Start(); err != nil {
			return fmt.Errorf("failed to start daemon: %w", err)
		}

		// Wait briefly and check if it's running
		time.Sleep(500 * time.Millisecond)
		if isRunning, pid, _ := getDaemonStatus(); isRunning {
			fmt.Printf("Daemon started successfully (PID: %d)\n", pid)
			return nil
		}
		return fmt.Errorf("daemon failed to start")
	} else {
		// Run in foreground
		cmd := exec.Command(daemonBinary, args...)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Stdin = os.Stdin
		
		return cmd.Run()
	}
}

// stopDaemonProcess stops the daemon process
func stopDaemonProcess(force bool) error {
	pidFile := getPIDFilePath()
	
	// Read PID file
	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("daemon is not running (no PID file)")
		}
		return fmt.Errorf("failed to read PID file: %w", err)
	}

	var pid int
	if _, err := fmt.Sscanf(string(pidBytes), "%d", &pid); err != nil || pid <= 0 {
		return fmt.Errorf("invalid PID in file: %s", string(pidBytes))
	}

	// Find the process
	process, err := os.FindProcess(pid)
	if err != nil {
		return fmt.Errorf("failed to find process: %w", err)
	}

	// Send termination signal
	signal := syscall.SIGTERM
	if force {
		signal = syscall.SIGKILL
	}

	if err := process.Signal(signal); err != nil {
		return fmt.Errorf("failed to send signal to process: %w", err)
	}

	// Wait for process to exit (unless force kill)
	if !force {
		for i := 0; i < 10; i++ {
			if err := process.Signal(syscall.Signal(0)); err != nil {
				// Process has exited
				os.Remove(pidFile)
				return nil
			}
			time.Sleep(time.Second)
		}
		
		// Force kill after timeout
		if err := process.Kill(); err != nil {
			return fmt.Errorf("failed to force kill process: %w", err)
		}
	}

	os.Remove(pidFile)
	return nil
}

// getDaemonStatus checks if the daemon is running
func getDaemonStatus() (bool, int, error) {
	pidFile := getPIDFilePath()
	
	// Read PID file
	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		if os.IsNotExist(err) {
			return false, 0, nil
		}
		return false, 0, fmt.Errorf("failed to read PID file: %w", err)
	}

	var pid int
	if _, err := fmt.Sscanf(string(pidBytes), "%d", &pid); err != nil || pid <= 0 {
		return false, 0, fmt.Errorf("invalid PID in file: %s", string(pidBytes))
	}

	// Check if process is alive
	process, err := os.FindProcess(pid)
	if err != nil {
		return false, pid, nil
	}

	if err := process.Signal(syscall.Signal(0)); err != nil {
		// Process is not alive, clean up stale PID file
		os.Remove(pidFile)
		return false, pid, nil
	}

	return true, pid, nil
}

// getPIDFilePath returns the path to the daemon PID file
func getPIDFilePath() string {
	dataDir := os.Getenv("CLIPMAN_DATA_DIR")
	if dataDir == "" {
		dataDir = filepath.Join(os.Getenv("HOME"), ".local", "share", "clipman")
	}
	return filepath.Join(dataDir, "run", "clipman.pid")
}

// sendIPCRequest sends a request to the running daemon via IPC
func sendIPCRequest(command string, args map[string]interface{}) (*ipc.Response, error) {
	req := &ipc.Request{
		Command: command,
		Args:    args,
	}
	
	return ipc.SendRequest("", req)
}

// RunForeground runs the daemon in foreground mode (for backward compatibility)
func RunForeground() error {
	return startDaemonProcess(false)
}

// Kill stops the daemon process (for backward compatibility)
func Kill() error {
	return stopDaemonProcess(false)
}

// Status returns daemon status (for backward compatibility)
func Status() (bool, error) {
	isRunning, _, err := getDaemonStatus()
	return isRunning, err
} 