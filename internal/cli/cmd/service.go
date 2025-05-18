package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
	"github.com/your-project/platform"
)

var (
	systemService bool
	forceService  bool
)

// serviceCmd represents the service command for managing system service
var serviceCmd = &cobra.Command{
	Use:   "service [install|uninstall|start|stop|status]",
	Short: "Manage Clipman system service",
	Long: `Manage the Clipman clipboard manager as a system service.

This command helps you install, uninstall, start, stop, and check the status
of the Clipman service on your system.

Examples:
  # Install Clipman as a user service
  clipman service install

  # Install Clipman as a system service (requires admin/root)
  clipman service install --system

  # Check service status
  clipman service status

  # Uninstall the service
  clipman service uninstall`,
	Args: cobra.ExactValidArgs(1),
	ValidArgs: []string{"install", "uninstall", "start", "stop", "status"},
	RunE: func(cmd *cobra.Command, args []string) error {
		action := args[0]
		
		switch action {
		case "install":
			return installService()
		case "uninstall":
			return uninstallService()
		case "start":
			return startService()
		case "stop":
			return stopService()
		case "status":
			return checkServiceStatus()
		default:
			return fmt.Errorf("invalid action: %s", action)
		}
	},
}

func init() {
	serviceCmd.Flags().BoolVar(&systemService, "system", false, "Install as system service (requires admin/root)")
	serviceCmd.Flags().BoolVar(&forceService, "force", false, "Force service operation even if it might overwrite existing files")
}

// installService installs Clipman as a system or user service
func installService() error {
	zapLogger.Info("Installing Clipman service", zap.Bool("system", systemService))

	// Check for root/admin if system service
	if systemService && (runtime.GOOS == "linux" || runtime.GOOS == "darwin") && os.Geteuid() != 0 {
		err := errors.New("must be run as root/admin to install system service")
		fmt.Fprintln(os.Stderr, err)
		zapLogger.Error("Permission denied", zap.Error(err))
		return err
	}

	// Get the service file path and content for the current OS
	servicePath, serviceContent, err := getServiceFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to prepare service file: %v\n", err)
		zapLogger.Error("Failed to prepare service file", zap.Error(err))
		return err
	}

	// Check if service already exists
	if fileExists(servicePath) && !forceService {
		msg := fmt.Sprintf("Service already exists at %s. Use --force to overwrite.", servicePath)
		fmt.Fprintln(os.Stderr, msg)
		zapLogger.Warn(msg)
		return errors.New(msg)
	}

	// Create directory if it doesn't exist
	serviceDir := filepath.Dir(servicePath)
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create service directory: %v\n", err)
		zapLogger.Error("Failed to create service directory", zap.Error(err))
		return err
	}

	// Atomic write: write to temp file, then move
	tmpFile := servicePath + ".tmp"
	if err := os.WriteFile(tmpFile, []byte(serviceContent), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write service file: %v\n", err)
		zapLogger.Error("Failed to write service file", zap.Error(err))
		return err
	}
	if err := os.Rename(tmpFile, servicePath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to move service file into place: %v\n", err)
		zapLogger.Error("Failed to move service file", zap.Error(err))
		return err
	}

	zapLogger.Info("Service file created", zap.String("path", servicePath))
	fmt.Printf("Service file created at %s\n", servicePath)

	// Reload and enable the service based on OS
	if err := enableService(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to enable service: %v\n", err)
		zapLogger.Error("Failed to enable service", zap.Error(err))
		return err
	}

	zapLogger.Info("Clipman service installed and enabled", zap.String("path", servicePath))
	fmt.Println("Clipman service installed and enabled.")
	return nil
}

// uninstallService removes the Clipman service
func uninstallService() error {
	zapLogger.Info("Uninstalling Clipman service")

	// Check for root/admin if system service
	if systemService && (runtime.GOOS == "linux" || runtime.GOOS == "darwin") && os.Geteuid() != 0 {
		err := errors.New("must be run as root/admin to uninstall system service")
		fmt.Fprintln(os.Stderr, err)
		zapLogger.Error("Permission denied", zap.Error(err))
		return err
	}

	// Get the service file path for the current OS
	servicePath, _, err := getServiceFile()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to determine service file: %v\n", err)
		zapLogger.Error("Failed to determine service file", zap.Error(err))
		return err
	}

	// Check if service exists
	if !fileExists(servicePath) {
		msg := fmt.Sprintf("Service does not exist at %s", servicePath)
		fmt.Println(msg)
		zapLogger.Info(msg)
		return nil
	}

	// Stop the service first
	if err := stopService(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to stop service before uninstall: %v\n", err)
		zapLogger.Warn("Failed to stop service before uninstall", zap.Error(err))
		// Continue with uninstall even if stop fails
	}

	// Disable the service based on OS
	if err := disableService(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to disable service: %v\n", err)
		zapLogger.Warn("Failed to disable service", zap.Error(err))
		// Continue with uninstall even if disable fails
	}

	// Remove the service file
	if err := os.Remove(servicePath); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to remove service file: %v\n", err)
		zapLogger.Error("Failed to remove service file", zap.Error(err))
		return err
	}

	zapLogger.Info("Clipman service has been uninstalled")
	fmt.Println("Clipman service has been uninstalled.")
	return nil
}

// startService starts the Clipman service (manual daemonization by default)
func startService() error {
	zapLogger.Info("Starting Clipman service")

	if systemService {
		// System service: use systemd/launchd/Windows service as before
		return startSystemService()
	}

	// Manual daemonization (default)
	executable, err := os.Executable()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get executable path: %v\n", err)
		zapLogger.Error("Failed to get executable path", zap.Error(err))
		return err
	}
	workDir, _ := os.Getwd()
	// TODO: Load config to get dataDir, LaunchAtStartup, LaunchOnLogin
	dataDir := filepath.Join(os.Getenv("HOME"), ".local/share/clipman")

	daemonizer := platform.GetPlatformDaemonizer()
	pid, err := daemonizer.Daemonize(executable, []string{}, workDir, dataDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start daemon: %v\n", err)
		zapLogger.Error("Failed to start daemon", zap.Error(err))
		return err
	}
	fmt.Printf("Clipman daemon started with PID %d\n", pid)
	zapLogger.Info("Clipman daemon started", zap.Int("pid", pid))

	// (Stub) If config.LaunchAtStartup or config.LaunchOnLogin, register for startup
	// TODO: Implement platform-specific registration for startup/login
	// if cfg.LaunchAtStartup || cfg.LaunchOnLogin { ... }

	return nil
}

// stopService stops the Clipman service (manual daemonization by default)
func stopService() error {
	zapLogger.Info("Stopping Clipman service")

	if systemService {
		// System service: use systemd/launchd/Windows service as before
		return stopSystemService()
	}

	// Manual daemonization: stop by reading PID file and killing process
	dataDir := filepath.Join(os.Getenv("HOME"), ".local/share/clipman")
	pidFile := filepath.Join(dataDir, "run", "clipman.pid")
	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to read PID file: %v\n", err)
		zapLogger.Error("Failed to read PID file", zap.Error(err))
		return err
	}
	var pid int
	_, err = fmt.Sscanf(string(pidBytes), "%d", &pid)
	if err != nil || pid <= 0 {
		fmt.Fprintf(os.Stderr, "Invalid PID in file: %s\n", string(pidBytes))
		zapLogger.Error("Invalid PID in file", zap.String("pid", string(pidBytes)))
		return fmt.Errorf("invalid PID in file")
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to find process: %v\n", err)
		zapLogger.Error("Failed to find process", zap.Error(err))
		return err
	}
	if err := proc.Kill(); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to kill process: %v\n", err)
		zapLogger.Error("Failed to kill process", zap.Error(err))
		return err
	}
	os.Remove(pidFile)
	fmt.Println("Clipman daemon stopped.")
	zapLogger.Info("Clipman daemon stopped", zap.Int("pid", pid))
	return nil
}

// checkServiceStatus checks the status of the Clipman service (manual daemonization by default)
func checkServiceStatus() error {
	zapLogger.Info("Checking Clipman service status")

	if systemService {
		// System service: use systemd/launchd/Windows service as before
		return checkSystemServiceStatus()
	}

	// Manual daemonization: check PID file and process
	dataDir := filepath.Join(os.Getenv("HOME"), ".local/share/clipman")
	pidFile := filepath.Join(dataDir, "run", "clipman.pid")
	pidBytes, err := os.ReadFile(pidFile)
	if err != nil {
		fmt.Println("Clipman daemon is not running (no PID file found).")
		zapLogger.Info("No PID file found, daemon not running")
		return nil
	}
	var pid int
	_, err = fmt.Sscanf(string(pidBytes), "%d", &pid)
	if err != nil || pid <= 0 {
		fmt.Println("Clipman daemon is not running (invalid PID file).")
		zapLogger.Info("Invalid PID file, daemon not running")
		return nil
	}
	proc, err := os.FindProcess(pid)
	if err != nil {
		fmt.Println("Clipman daemon is not running (process not found).")
		zapLogger.Info("Process not found, daemon not running")
		return nil
	}
	// Try to signal the process (0 = check existence)
	if err := proc.Signal(os.Signal(syscall.Signal(0))); err != nil {
		fmt.Println("Clipman daemon is not running (process not alive).")
		zapLogger.Info("Process not alive, daemon not running")
		return nil
	}
	fmt.Printf("Clipman daemon is running with PID %d.\n", pid)
	zapLogger.Info("Daemon is running", zap.Int("pid", pid))
	return nil
}

// parseServiceStatus parses raw service status output and returns a user-friendly summary
func parseServiceStatus(output, goos string) string {
	output = strings.ToLower(output)
	if goos == "windows" {
		if strings.Contains(output, "running") {
			return "Clipman service is running."
		} else if strings.Contains(output, "stopped") {
			return "Clipman service is stopped."
		}
		return "Clipman service status unknown. Raw output:\n" + output
	}
	if goos == "darwin" {
		if strings.Contains(output, "pid") {
			return "Clipman service is running."
		} else if strings.Contains(output, "could not find service") || strings.Contains(output, "no such process") {
			return "Clipman service is not running."
		}
		return "Clipman service status unknown. Raw output:\n" + output
	}
	// Linux/systemd
	if strings.Contains(output, "active (running)") {
		return "Clipman service is running."
	} else if strings.Contains(output, "inactive (dead)") || strings.Contains(output, "could not be found") {
		return "Clipman service is not running."
	}
	return "Clipman service status unknown. Raw output:\n" + output
}

// getServiceFile returns the path and content of the service file based on OS
func getServiceFile() (string, string, error) {
	// Get the full path to the current executable
	executable, err := os.Executable()
	if err != nil {
		return "", "", fmt.Errorf("failed to get executable path: %v", err)
	}
	
	switch runtime.GOOS {
	case "windows":
		// For Windows, use sc.exe to create a service
		return getWindowsServiceInfo(executable)
	case "darwin":
		// For macOS, use launchd
		return getDarwinServiceInfo(executable)
	default:
		// For Linux, use systemd
		return getLinuxServiceInfo(executable)
	}
}

// getWindowsServiceInfo returns service info for Windows
func getWindowsServiceInfo(executable string) (string, string, error) {
	servicePath := filepath.Join(os.Getenv("TEMP"), "clipman-service.bat")
	
	// Create a batch script that will be used to create the service
	serviceContent := fmt.Sprintf(`@echo off
sc create clipman binPath= "%s --detach" start= auto DisplayName= "Clipman Clipboard Manager"
sc description clipman "Clipboard manager that provides history and synchronization capabilities"
sc start clipman
`, executable)
	
	return servicePath, serviceContent, nil
}

// getDarwinServiceInfo returns service info for macOS
func getDarwinServiceInfo(executable string) (string, string, error) {
	var servicePath string
	if systemService {
		servicePath = "/Library/LaunchDaemons/com.clipman.daemon.plist"
	} else {
		servicePath = filepath.Join(os.Getenv("HOME"), "Library/LaunchAgents/com.clipman.daemon.plist")
	}
	
	// Base paths for data/logs
	dataDir := filepath.Join(os.Getenv("HOME"), "Library/Application Support/Clipman")
	logDir := filepath.Join(dataDir, "logs")
	
	// Ensure directories exist
	os.MkdirAll(dataDir, 0755)
	os.MkdirAll(logDir, 0755)
	
	serviceContent := fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>com.clipman.daemon</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
	<key>KeepAlive</key>
	<true/>
	<key>StandardOutPath</key>
	<string>%s/clipman.log</string>
	<key>StandardErrorPath</key>
	<string>%s/clipman-error.log</string>
</dict>
</plist>
`, executable, logDir, logDir)
	
	return servicePath, serviceContent, nil
}

// getLinuxServiceInfo returns service info for Linux systems
func getLinuxServiceInfo(executable string) (string, string, error) {
	var servicePath string
	if systemService {
		servicePath = "/etc/systemd/system/clipman.service"
	} else {
		// Create user systemd directory if it doesn't exist
		userSystemdDir := filepath.Join(os.Getenv("HOME"), ".config/systemd/user")
		os.MkdirAll(userSystemdDir, 0755)
		servicePath = filepath.Join(userSystemdDir, "clipman.service")
	}
	
	// Create service file content
	var serviceContent string
	if systemService {
		serviceContent = fmt.Sprintf(`[Unit]
Description=Clipman Clipboard Manager
After=network.target

[Service]
ExecStart=%s
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
`, executable)
	} else {
		serviceContent = fmt.Sprintf(`[Unit]
Description=Clipman Clipboard Manager
After=graphical-session.target

[Service]
ExecStart=%s
Restart=on-failure
RestartSec=5

[Install]
WantedBy=default.target
`, executable)
	}
	
	return servicePath, serviceContent, nil
}

// enableService enables the service based on OS
func enableService() error {
	var cmd *exec.Cmd
	
	switch runtime.GOOS {
	case "windows":
		// For Windows, the batch script already handles enabling
		return nil
	case "darwin":
		if systemService {
			cmd = exec.Command("launchctl", "load", "-w", "/Library/LaunchDaemons/com.clipman.daemon.plist")
		} else {
			cmd = exec.Command("launchctl", "load", "-w", filepath.Join(os.Getenv("HOME"), "Library/LaunchAgents/com.clipman.daemon.plist"))
		}
	default: // Linux and others
		if systemService {
			cmd = exec.Command("systemctl", "daemon-reload")
			if output, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to reload systemd: %v\n%s", err, output)
			}
			cmd = exec.Command("systemctl", "enable", "--now", "clipman.service")
		} else {
			cmd = exec.Command("systemctl", "--user", "daemon-reload")
			if output, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("failed to reload user systemd: %v\n%s", err, output)
			}
			cmd = exec.Command("systemctl", "--user", "enable", "--now", "clipman.service")
		}
	}
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to enable service: %v\n%s", err, output)
	}
	
	return nil
}

// disableService disables the service based on OS
func disableService() error {
	var cmd *exec.Cmd
	
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("sc", "delete", "clipman")
	case "darwin":
		// For macOS, unloading the service is handled in stopService
		return nil
	default: // Linux and others
		if systemService {
			cmd = exec.Command("systemctl", "disable", "clipman.service")
		} else {
			cmd = exec.Command("systemctl", "--user", "disable", "clipman.service")
		}
	}
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to disable service: %v\n%s", err, output)
	}
	
	return nil
}

// fileExists checks if a file exists
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
} 