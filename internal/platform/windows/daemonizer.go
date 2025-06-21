//go:build windows
// +build windows

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"

	"github.com/berrythewa/clipman-daemon/internal/common"
	"github.com/berrythewa/clipman-daemon/internal/config"
	"go.uber.org/zap"
	"golang.org/x/sys/windows"
)

// WindowsDaemonizer implements platform-specific daemonization for Windows
type WindowsDaemonizer struct{}

// NewDaemonizer creates a new platform-specific daemonizer implementation
func NewDaemonizer() *WindowsDaemonizer {
	return &WindowsDaemonizer{}
}

var (
	modkernel32 = windows.NewLazySystemDLL("kernel32.dll")
	procCreateMutexW = modkernel32.NewProc("CreateMutexW")
)

func createNamedMutex(name string) (windows.Handle, error) {
	namePtr, _ := windows.UTF16PtrFromString(name)
	handle, _, err := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(namePtr)))
	if handle == 0 {
		return 0, err
	}
	return windows.Handle(handle), nil
}

// setupLogging initializes zap logger and log file for daemon output.
func (d *WindowsDaemonizer) setupLogging(cfg *config.Config) (*zap.Logger, *os.File, error) {
	// Use common logger for platform operations
	logger, err := common.NewPlatformLogger(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to setup logging: %v", err)
	}

	// For daemonizers, we also need a file handle for stdout/stderr redirection
	logDir := cfg.SystemPaths.LogDir
	logFile := filepath.Join(logDir, "clipman_daemon.log")
	logF, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open log file: %v", err)
	}

	return logger, logF, nil
}

// Daemonize forks the current process and runs it in the background
func (d *WindowsDaemonizer) Daemonize(executable string, args []string, workDir string, dataDir string) (int, error) {
	// Create a named mutex to ensure only one instance runs
	mutex, err := createNamedMutex("Global\\ClipmanDaemonMutex")
	if err != nil {
		return 0, fmt.Errorf("failed to create mutex: %w", err)
	}
	defer windows.CloseHandle(mutex)

	// Load config
	cfg, err := config.Load("")
	if err != nil {
		return 0, fmt.Errorf("failed to load config: %v", err)
	}

	// Setup logging
	logger, logF, err := d.setupLogging(cfg)
	if err != nil {
		return 0, fmt.Errorf("failed to setup logging: %v", err)
	}
	defer func() {
		logF.Close()
		logger.Sync()
	}()

	logger.Info("Starting daemonization", zap.String("executable", executable), zap.Strings("args", args))

	// Remove the --detach flag to prevent infinite recursion
	filteredArgs := make([]string, 0, len(args))
	for _, arg := range args {
		if arg != "--detach" {
			filteredArgs = append(filteredArgs, arg)
		}
	}
	logger.Debug("Filtered args", zap.Strings("filteredArgs", filteredArgs))

	// Prepare the command to run
	cmd := exec.Command(executable, filteredArgs...)
	cmd.Dir = workDir

	// Redirect stdout and stderr to log file
	cmd.Stdout = logF
	cmd.Stderr = logF
	cmd.Stdin = nil // No input

	// Set key environment variables to indicate we're running in daemon mode
	newEnv := os.Environ()
	newEnv = append(newEnv, "CLIPMAN_DAEMON=1")
	cmd.Env = newEnv

	// Windows-specific: Hide window and set process creation flags
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow: true,
		CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.DETACHED_PROCESS,
	}

	// Write PID file for the daemon
	pidDir := filepath.Join(dataDir, "run")
	logger.Info("Ensuring PID directory exists", zap.String("pidDir", pidDir))
	if err := os.MkdirAll(pidDir, 0755); err != nil {
		logger.Error("Failed to create pid directory", zap.Error(err))
		return 0, fmt.Errorf("failed to create pid directory: %v", err)
	}

	pidFile := filepath.Join(pidDir, "clipman.pid")

	// Start the process
	logger.Info("Starting daemon process", zap.String("executable", executable), zap.Strings("args", filteredArgs))
	if err := cmd.Start(); err != nil {
		logger.Error("Failed to start daemon process", zap.Error(err))
		return 0, fmt.Errorf("failed to start daemon process: %v", err)
	}

	// Write the PID to the file
	pid := cmd.Process.Pid
	logger.Info("Writing PID file", zap.String("pidFile", pidFile), zap.Int("pid", pid))
	if err := os.WriteFile(pidFile, []byte(fmt.Sprintf("%d", pid)), 0644); err != nil {
		logger.Error("Failed to write pid file", zap.Error(err), zap.String("pidFile", pidFile))
		return pid, fmt.Errorf("failed to write pid file: %v", err)
	}

	// Detach the process
	if err := cmd.Process.Release(); err != nil {
		logger.Error("Failed to release daemon process", zap.Error(err))
		return pid, fmt.Errorf("failed to release daemon process: %v", err)
	}

	logger.Info("Daemon started successfully", zap.Int("pid", pid), zap.String("pidFile", pidFile))
	return pid, nil
}

// IsRunningAsDaemon returns true if the current process is running as a daemon
func (d *WindowsDaemonizer) IsRunningAsDaemon() bool {
	// Check if we're running as a Windows service
	if isWindowsService() {
		return true
	}

	// Check if we're running in a detached process
	ppid := os.Getppid()
	if ppid <= 4 { // System, smss.exe, csrss.exe, or wininit.exe
		return true
	}

	// Check if we're running in a new process group
	pid := os.Getpid()
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		return false
	}
	return pid == pgid
}

// isWindowsService checks if the current process is running as a Windows service
func isWindowsService() bool {
	// This is a simplified check - in a real implementation, you'd use the Windows API
	// to check if the process is running as a service
	return false
} 