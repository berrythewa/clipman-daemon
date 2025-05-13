//go:build darwin
// +build darwin

package platform

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
    "github.com/berrythewa/clipman-daemon/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// DarwinDaemonizer implements platform-specific daemonization for macOS
type DarwinDaemonizer struct{}

// NewDaemonizer creates a new platform-specific daemonizer implementation
func NewDaemonizer() *DarwinDaemonizer {
	return &DarwinDaemonizer{}
}

// setupLogging initializes zap logger and log file for daemon output.
func (d *DarwinDaemonizer) setupLogging(cfg *config.Config) (*zap.Logger, *os.File, error) {
	// 1. Get log directory and ensure it exists
	logDir := cfg.SystemPaths.LogDir
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return nil, nil, fmt.Errorf("failed to create log directory: %v", err)
	}

	// 2. Build log file path
	logFile := filepath.Join(logDir, "clipman_daemon.log")

	// 3. Open log file for append/write
	logF, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open log file: %v", err)
	}

	// 4. Initialize zap logger to log file
	fileSyncer := zapcore.AddSync(logF)
	encoderCfg := zap.NewProductionEncoderConfig()
	encoderCfg.TimeKey = "ts"
	encoderCfg.EncodeTime = zapcore.ISO8601TimeEncoder
	core := zapcore.NewCore(
		zapcore.NewConsoleEncoder(encoderCfg),
		fileSyncer,
		zap.InfoLevel,
	)
	logger := zap.New(core)

	return logger, logF, nil
}

// Daemonize forks the current process and runs it in the background
func (d *DarwinDaemonizer) Daemonize(executable string, args []string, workDir string, dataDir string) (int, error) {
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

	// Redirect stdin to /dev/null
	nullDev, err := os.OpenFile("/dev/null", os.O_RDWR, 0)
	if err != nil {
		logger.Error("Failed to open /dev/null", zap.Error(err))
		return 0, fmt.Errorf("failed to open /dev/null: %v", err)
	}
	defer nullDev.Close()
	cmd.Stdin = nullDev

	// Detach from process group (Unix-specific)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setsid: true,
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

	logger.Info("Daemon started successfully", zap.Int("pid", pid), zap.String("pidFile", pidFile))
	return pid, nil
}

// IsRunningAsDaemon returns true if the current process is running as a daemon
func (d *DarwinDaemonizer) IsRunningAsDaemon() bool {
	// Check if we're a session leader (setsid was called)
	pid := os.Getpid()
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		return false
	}
	
	// If we're not the session leader, we're not a daemon
	if pid != pgid {
		return false
	}
	
	// Check if our ppid is 1 (launchd on macOS)
	ppid := os.Getppid()
	return ppid == 1
} 