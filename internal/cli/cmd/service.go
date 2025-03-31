package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
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
	
	// Get the service file path and content for the current OS
	servicePath, serviceContent, err := getServiceFile()
	if err != nil {
		return fmt.Errorf("failed to prepare service file: %v", err)
	}
	
	// Check if service already exists
	if fileExists(servicePath) && !forceService {
		return fmt.Errorf("service already exists at %s. Use --force to overwrite", servicePath)
	}
	
	// Create directory if it doesn't exist
	serviceDir := filepath.Dir(servicePath)
	if err := os.MkdirAll(serviceDir, 0755); err != nil {
		return fmt.Errorf("failed to create service directory: %v", err)
	}
	
	// Write the service file
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %v", err)
	}
	
	zapLogger.Info("Service file created", zap.String("path", servicePath))
	
	// Reload and enable the service based on OS
	if err := enableService(); err != nil {
		zapLogger.Error("Failed to enable service", zap.Error(err))
		return err
	}
	
	zapLogger.Info("Clipman service installed and enabled", zap.String("path", servicePath))
	return nil
}

// uninstallService removes the Clipman service
func uninstallService() error {
	zapLogger.Info("Uninstalling Clipman service")
	
	// Get the service file path for the current OS
	servicePath, _, err := getServiceFile()
	if err != nil {
		return fmt.Errorf("failed to determine service file: %v", err)
	}
	
	// Check if service exists
	if !fileExists(servicePath) {
		zapLogger.Info("Service does not exist", zap.String("path", servicePath))
		return nil
	}
	
	// Stop the service first
	if err := stopService(); err != nil {
		zapLogger.Warn("Failed to stop service before uninstall", zap.Error(err))
		// Continue with uninstall even if stop fails
	}
	
	// Disable the service based on OS
	if err := disableService(); err != nil {
		zapLogger.Warn("Failed to disable service", zap.Error(err))
		// Continue with uninstall even if disable fails
	}
	
	// Remove the service file
	if err := os.Remove(servicePath); err != nil {
		return fmt.Errorf("failed to remove service file: %v", err)
	}
	
	zapLogger.Info("Clipman service has been uninstalled")
	return nil
}

// startService starts the Clipman service
func startService() error {
	zapLogger.Info("Starting Clipman service")
	
	var cmd *exec.Cmd
	
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("sc", "start", "clipman")
	case "darwin":
		if systemService {
			cmd = exec.Command("launchctl", "load", "/Library/LaunchDaemons/com.clipman.daemon.plist")
		} else {
			cmd = exec.Command("launchctl", "load", filepath.Join(os.Getenv("HOME"), "Library/LaunchAgents/com.clipman.daemon.plist"))
		}
	default: // Linux and others
		if systemService {
			cmd = exec.Command("systemctl", "start", "clipman.service")
		} else {
			cmd = exec.Command("systemctl", "--user", "start", "clipman.service")
		}
	}
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to start service: %v\n%s", err, output)
	}
	
	zapLogger.Info("Clipman service started")
	return nil
}

// stopService stops the Clipman service
func stopService() error {
	zapLogger.Info("Stopping Clipman service")
	
	var cmd *exec.Cmd
	
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("sc", "stop", "clipman")
	case "darwin":
		if systemService {
			cmd = exec.Command("launchctl", "unload", "/Library/LaunchDaemons/com.clipman.daemon.plist")
		} else {
			cmd = exec.Command("launchctl", "unload", filepath.Join(os.Getenv("HOME"), "Library/LaunchAgents/com.clipman.daemon.plist"))
		}
	default: // Linux and others
		if systemService {
			cmd = exec.Command("systemctl", "stop", "clipman.service")
		} else {
			cmd = exec.Command("systemctl", "--user", "stop", "clipman.service")
		}
	}
	
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to stop service: %v\n%s", err, output)
	}
	
	zapLogger.Info("Clipman service stopped")
	return nil
}

// checkServiceStatus checks the status of the Clipman service
func checkServiceStatus() error {
	zapLogger.Info("Checking Clipman service status")
	
	var cmd *exec.Cmd
	
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("sc", "query", "clipman")
	case "darwin":
		if systemService {
			cmd = exec.Command("launchctl", "list", "com.clipman.daemon")
		} else {
			cmd = exec.Command("launchctl", "list", "com.clipman.daemon")
		}
	default: // Linux and others
		if systemService {
			cmd = exec.Command("systemctl", "status", "clipman.service")
		} else {
			cmd = exec.Command("systemctl", "--user", "status", "clipman.service")
		}
	}
	
	output, err := cmd.CombinedOutput()
	fmt.Println(string(output))
	
	// Don't return an error if the command returns non-zero (service might not be running)
	if err != nil {
		zapLogger.Info("Service status check returned non-zero", zap.Error(err))
	}
	
	return nil
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