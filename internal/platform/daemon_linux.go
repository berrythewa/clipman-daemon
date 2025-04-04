//go:build linux
// +build linux

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
)

// LinuxDaemonizer implements platform-specific daemonization for Linux
type LinuxDaemonizer struct{}

// NewDaemonizer creates a new platform-specific daemonizer implementation
func NewDaemonizer() *LinuxDaemonizer {
	return &LinuxDaemonizer{}
}

// Daemonize forks the current process and runs it in the background
func (d *LinuxDaemonizer) Daemonize(executable string, args []string, workDir string, dataDir string) (int, error) {
	// Remove the --detach flag to prevent infinite recursion
	filteredArgs := make([]string, 0, len(args))
	for _, arg := range args {
		if arg != "--detach" {
			filteredArgs = append(filteredArgs, arg)
		}
	}
	
	// Create log files for the daemon's stdout and stderr
	// This ensures that the output is captured even when running in the background
	logDir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create log directory: %v", err)
	}
	
	stdoutPath := filepath.Join(logDir, "clipman-daemon.log")
	stderrPath := filepath.Join(logDir, "clipman-daemon-error.log")
	
	// Open log files with append mode
	stdout, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return 0, fmt.Errorf("failed to open stdout log file: %v", err)
	}
	defer stdout.Close()
	
	stderr, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return 0, fmt.Errorf("failed to open stderr log file: %v", err)
	}
	defer stderr.Close()

	// Prepare the command to run
	cmd := exec.Command(executable, filteredArgs...)
	cmd.Dir = workDir
	
	// Redirect standard file descriptors
	cmd.Stdin = nil // No input
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	
	// Set key environment variables to indicate we're running in daemon mode
	newEnv := os.Environ()
	newEnv = append(newEnv, "CLIPMAN_DAEMON=1")
	cmd.Env = newEnv
	
	// Critical: Detach from process group and create a new session
	// This is what allows the process to continue running after parent exits
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid:     true,       // Create a new session
		Foreground: false,      // Run in background
		// Prevent child from being killed when parent is
		Pgid:    0,             // New process group
	}

	// Write PID file for the daemon
	pidDir := filepath.Join(dataDir, "run")
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create pid directory: %v", err)
	}
	
	pidFile := filepath.Join(pidDir, "clipman.pid")
	
	// Start the process
	if err := cmd.Start(); err != nil {
		return 0, fmt.Errorf("failed to start daemon process: %v", err)
	}
	
	// Write the PID to the file
	pid := cmd.Process.Pid
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
		return pid, fmt.Errorf("failed to write pid file: %v", err)
	}
	
	// Detach the process - this is critical to prevent zombie processes
	// We're explicitly NOT calling cmd.Wait() since we want the process to continue independently
	if err := cmd.Process.Release(); err != nil {
		return pid, fmt.Errorf("failed to release daemon process: %v", err)
	}
	
	// Give the daemon a moment to initialize
	time.Sleep(100 * time.Millisecond)
	
	// Verify the process is still running
	if err := syscall.Kill(pid, 0); err != nil {
		return pid, fmt.Errorf("daemon process failed to start (PID: %d): %v", pid, err)
	}

	return pid, nil
}

// IsRunningAsDaemon returns true if the current process is running as a daemon
func (d *LinuxDaemonizer) IsRunningAsDaemon() bool {
	// Check if we're a session leader (setsid was called)
	// In Linux, if the process is a session leader, its process group ID equals its PID
	pid := os.Getpid()
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		return false
	}
	
	// If we're not the session leader, we're not a daemon
	if pid != pgid {
		return false
	}
	
	// Check if our ppid is 1 (init) or systemd
	ppid := os.Getppid()
	return ppid == 1 || ppid == 0
} 