//go:build linux
// +build linux

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
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

	// Prepare the command to run
	cmd := exec.Command(executable, filteredArgs...)
	cmd.Dir = workDir

	// Redirect standard file descriptors to /dev/null
	nullDev, err := os.OpenFile("/dev/null", os.O_RDWR, 0)
	if err != nil {
		return 0, fmt.Errorf("failed to open /dev/null: %v", err)
	}
	defer nullDev.Close()
	
	cmd.Stdin = nullDev
	cmd.Stdout = nullDev
	cmd.Stderr = nullDev
	
	// Detach from process group (Unix-specific)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
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