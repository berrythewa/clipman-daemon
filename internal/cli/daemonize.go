// Package cli implements the command-line interface for Clipman
package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"syscall"
)

// daemonize forks the current process and runs it in the background
// This is a simplified version - platform-specific implementations may be added later
func daemonize() error {
	// Get the full path to the executable
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %v", err)
	}

	// Get the current working directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("failed to get current working directory: %v", err)
	}

	// Prepare the command to run
	cmd := exec.Command(executable, os.Args[1:]...)
	
	// Remove the --detach flag to prevent infinite recursion
	for i, arg := range cmd.Args {
		if arg == "--detach" {
			cmd.Args = append(cmd.Args[:i], cmd.Args[i+1:]...)
			break
		}
	}

	// Set the working directory
	cmd.Dir = cwd
	
	// Redirect standard file descriptors to /dev/null or equivalent
	if runtime.GOOS != "windows" {
		nullDev, err := os.OpenFile("/dev/null", os.O_RDWR, 0)
		if err != nil {
			return fmt.Errorf("failed to open /dev/null: %v", err)
		}
		defer nullDev.Close()
		
		cmd.Stdin = nullDev
		cmd.Stdout = nullDev
		cmd.Stderr = nullDev
		
		// Detach from process group
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setsid: true,
		}
	} else {
		// Windows-specific: hide the console window
		cmd.SysProcAttr = &syscall.SysProcAttr{
			HideWindow: true,
		}
	}

	// Write PID file for the daemon
	pidDir := filepath.Join(cfg.DataDir, "run")
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		return fmt.Errorf("failed to create pid directory: %v", err)
	}
	
	pidFile := filepath.Join(pidDir, "clipman.pid")
	
	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start daemon process: %v", err)
	}
	
	// Write the PID to the file
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", cmd.Process.Pid)), 0644); err != nil {
		return fmt.Errorf("failed to write pid file: %v", err)
	}

	fmt.Printf("Clipman started in background (PID: %d)\n", cmd.Process.Pid)
	
	return nil
} 