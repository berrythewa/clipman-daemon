//go:build windows
// +build windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

// WindowsDaemonizer implements platform-specific daemonization for Windows
type WindowsDaemonizer struct{}

// NewDaemonizer creates a new platform-specific daemonizer implementation
func NewDaemonizer() *WindowsDaemonizer {
	return &WindowsDaemonizer{}
}

// Daemonize forks the current process and runs it in the background
func (d *WindowsDaemonizer) Daemonize(executable string, args []string, workDir string, dataDir string) (int, error) {
	// Remove the --detach flag to prevent infinite recursion
	filteredArgs := make([]string, 0, len(args))
	for _, arg := range args {
		if arg != "--detach" {
			filteredArgs = append(filteredArgs, arg)
		}
	}

	// Prepare the command to run
	cmd := exec.Command(executable, filteredArgs...)
	cmd.Dir = workDir
	
	// Windows-specific: Hide window when running as a daemon
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
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

	return pid, nil
}

// IsRunningAsDaemon returns true if the current process is running as a daemon
func (d *WindowsDaemonizer) IsRunningAsDaemon() bool {
	// On Windows, this is trickier to determine
	// Without direct console attachment detection, we'll use a simple heuristic
	// A proper implementation would use more Windows-specific APIs
	
	// For now, check if we're being run by the Windows service manager
	ppid := os.Getppid()
	
	// This is a simplified heuristic
	// In reality, we'd check for services.exe parent or other Windows-specific methods
	return ppid <= 4 // System, smss.exe, csrss.exe, or wininit.exe are likely parents for services
} 