//go:build linux
// +build linux

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"syscall"
)

// LinuxDaemonizer implements the Daemonizer interface for Linux
type LinuxDaemonizer struct{}

// Daemonize forks the current process and runs it in the background
func (d *LinuxDaemonizer) Daemonize(executable string, args []string, workDir string, dataDir string) (int, error) {
	// Create run directory if it doesn't exist
	runDir := filepath.Join(dataDir, "run")
	if err := os.MkdirAll(runDir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create run directory: %w", err)
	}

	// Create PID file path
	pidFile := filepath.Join(runDir, "clipman.pid")

	// Check if daemon is already running
	if pid, err := d.getRunningPID(pidFile); err == nil {
		return 0, fmt.Errorf("daemon already running with PID %d", pid)
	}

	// Start the daemon process
	cmd := exec.Command(executable, args...)
	cmd.Dir = workDir
	cmd.Env = append(os.Environ(), "CLIPMAN_DAEMON=1")

	// Set up process group
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Start the process
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start daemon: %w", err)
	}

	// Write PID file
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(cmd.Process.Pid)), 0644); err != nil {
		// Try to kill the process if we can't write the PID file
		cmd.Process.Kill()
		return 0, fmt.Errorf("failed to write PID file: %w", err)
	}

	return cmd.Process.Pid, nil
}

// IsRunningAsDaemon returns true if the current process is running as a daemon
func (d *LinuxDaemonizer) IsRunningAsDaemon() bool {
	return os.Getenv("CLIPMAN_DAEMON") == "1"
}

// getRunningPID checks if a daemon is already running and returns its PID
func (d *LinuxDaemonizer) getRunningPID(pidFile string) (int, error) {
	data, err := os.ReadFile(pidFile)
	if err != nil {
		return 0, err
	}

	pid, err := strconv.Atoi(string(data))
	if err != nil {
		return 0, fmt.Errorf("invalid PID in file: %w", err)
	}

	// Check if process exists
	process, err := os.FindProcess(pid)
	if err != nil {
		return 0, err
	}

	// Send signal 0 to check if process is alive
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return 0, err
	}

	return pid, nil
} 