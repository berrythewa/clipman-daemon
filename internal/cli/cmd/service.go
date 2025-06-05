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

// newServiceCmd creates the service command
func newServiceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "service",
		Short: "Manage Clipman daemon as a system service",
		Long: `Manage the Clipman daemon as a system service:
  • Install as systemd service (Linux) or launchd service (macOS)
  • Start, stop, restart the service
  • Check service status
  • Enable/disable auto-start on boot
  • Uninstall the service`,
	}

	// Add subcommands
	cmd.AddCommand(newServiceInstallCmd())
	cmd.AddCommand(newServiceUninstallCmd())
	cmd.AddCommand(newServiceStartCmd())
	cmd.AddCommand(newServiceStopCmd())
	cmd.AddCommand(newServiceRestartCmd())
	cmd.AddCommand(newServiceStatusCmd())
	cmd.AddCommand(newServiceEnableCmd())
	cmd.AddCommand(newServiceDisableCmd())

	return cmd
}

func newServiceInstallCmd() *cobra.Command {
	var (
		system bool
		start  bool
		enable bool
	)

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install Clipman daemon as a service",
		Long: `Install the Clipman daemon as a system service.

By default, installs as a user service. Use --system to install as a system-wide service.
The --start flag will start the service immediately after installation.
The --enable flag will enable auto-start on boot (default: true).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := GetZapLogger()
			logger.Info("Installing Clipman daemon service", 
				zap.Bool("system", system), 
				zap.Bool("start", start),
				zap.Bool("enable", enable))

			if err := installService(system, enable); err != nil {
				return fmt.Errorf("failed to install service: %w", err)
			}

			if start {
				if err := startService(system); err != nil {
					logger.Warn("Service installed but failed to start", zap.Error(err))
					return fmt.Errorf("service installed but failed to start: %w", err)
				}
				fmt.Println("Service installed and started successfully")
			} else {
				fmt.Println("Service installed successfully")
				fmt.Println("Run 'clipman service start' to start the daemon")
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&system, "system", false, "install as system service (requires admin privileges)")
	cmd.Flags().BoolVar(&start, "start", false, "start the service after installation")
	cmd.Flags().BoolVar(&enable, "enable", true, "enable auto-start on boot")
	
	return cmd
}

func newServiceUninstallCmd() *cobra.Command {
	var system bool

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Uninstall Clipman daemon service",
		RunE: func(cmd *cobra.Command, args []string) error {
			logger := GetZapLogger()
			logger.Info("Uninstalling Clipman daemon service", zap.Bool("system", system))

			if err := uninstallService(system); err != nil {
				return fmt.Errorf("failed to uninstall service: %w", err)
			}

			fmt.Println("Service uninstalled successfully")
			return nil
		},
	}

	cmd.Flags().BoolVar(&system, "system", false, "uninstall system service (requires admin privileges)")
	return cmd
}

func newServiceStartCmd() *cobra.Command {
	var system bool

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start Clipman daemon service",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := startService(system); err != nil {
				return fmt.Errorf("failed to start service: %w", err)
			}
			fmt.Println("Service started successfully")
			return nil
		},
	}

	cmd.Flags().BoolVar(&system, "system", false, "manage system service")
	return cmd
}

func newServiceStopCmd() *cobra.Command {
	var system bool

	cmd := &cobra.Command{
		Use:   "stop",
		Short: "Stop Clipman daemon service",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := stopService(system); err != nil {
				return fmt.Errorf("failed to stop service: %w", err)
			}
			fmt.Println("Service stopped successfully")
			return nil
		},
	}

	cmd.Flags().BoolVar(&system, "system", false, "manage system service")
	return cmd
}

func newServiceRestartCmd() *cobra.Command {
	var system bool

	cmd := &cobra.Command{
		Use:   "restart",
		Short: "Restart Clipman daemon service",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := restartService(system); err != nil {
				return fmt.Errorf("failed to restart service: %w", err)
			}
			fmt.Println("Service restarted successfully")
			return nil
		},
	}

	cmd.Flags().BoolVar(&system, "system", false, "manage system service")
	return cmd
}

func newServiceStatusCmd() *cobra.Command {
	var system bool

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show Clipman daemon service status",
		RunE: func(cmd *cobra.Command, args []string) error {
			status, err := getServiceStatus(system)
			if err != nil {
				return fmt.Errorf("failed to get service status: %w", err)
			}
			fmt.Println(status)
			return nil
		},
	}

	cmd.Flags().BoolVar(&system, "system", false, "check system service")
	return cmd
}

func newServiceEnableCmd() *cobra.Command {
	var system bool

	cmd := &cobra.Command{
		Use:   "enable",
		Short: "Enable Clipman daemon service auto-start",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := enableService(system); err != nil {
				return fmt.Errorf("failed to enable service: %w", err)
			}
			fmt.Println("Service enabled for auto-start")
			return nil
		},
	}

	cmd.Flags().BoolVar(&system, "system", false, "manage system service")
	return cmd
}

func newServiceDisableCmd() *cobra.Command {
	var system bool

	cmd := &cobra.Command{
		Use:   "disable",
		Short: "Disable Clipman daemon service auto-start",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := disableService(system); err != nil {
				return fmt.Errorf("failed to disable service: %w", err)
			}
			fmt.Println("Service disabled from auto-start")
			return nil
		},
	}

	cmd.Flags().BoolVar(&system, "system", false, "manage system service")
	return cmd
}

// Platform-specific service management functions
func installService(system, enable bool) error {
	switch runtime.GOOS {
	case "linux":
		return installSystemdService(system, enable)
	case "darwin":
		return installLaunchdService(system, enable)
	case "windows":
		return installWindowsService(system, enable)
	default:
		return fmt.Errorf("service installation not supported on %s", runtime.GOOS)
	}
}

func uninstallService(system bool) error {
	switch runtime.GOOS {
	case "linux":
		return uninstallSystemdService(system)
	case "darwin":
		return uninstallLaunchdService(system)
	case "windows":
		return uninstallWindowsService(system)
	default:
		return fmt.Errorf("service uninstallation not supported on %s", runtime.GOOS)
	}
}

func startService(system bool) error {
	switch runtime.GOOS {
	case "linux":
		return controlSystemdService("start", system)
	case "darwin":
		return controlLaunchdService("load", system)
	case "windows":
		return controlWindowsService("start", system)
	default:
		return fmt.Errorf("service control not supported on %s", runtime.GOOS)
	}
}

func stopService(system bool) error {
	switch runtime.GOOS {
	case "linux":
		return controlSystemdService("stop", system)
	case "darwin":
		return controlLaunchdService("unload", system)
	case "windows":
		return controlWindowsService("stop", system)
	default:
		return fmt.Errorf("service control not supported on %s", runtime.GOOS)
	}
}

func restartService(system bool) error {
	switch runtime.GOOS {
	case "linux":
		return controlSystemdService("restart", system)
	case "darwin":
		// launchd doesn't have restart, so unload then load
		logger := GetZapLogger()
		if err := controlLaunchdService("unload", system); err != nil {
			logger.Warn("Failed to unload service for restart", zap.Error(err))
		}
		return controlLaunchdService("load", system)
	case "windows":
		return controlWindowsService("restart", system)
	default:
		return fmt.Errorf("service control not supported on %s", runtime.GOOS)
	}
}

func enableService(system bool) error {
	switch runtime.GOOS {
	case "linux":
		return controlSystemdService("enable", system)
	case "darwin":
		// launchd services are enabled by being present in the directory
		return nil
	case "windows":
		return controlWindowsService("enable", system)
	default:
		return fmt.Errorf("service control not supported on %s", runtime.GOOS)
	}
}

func disableService(system bool) error {
	switch runtime.GOOS {
	case "linux":
		return controlSystemdService("disable", system)
	case "darwin":
		// For launchd, we need to unload the service
		return controlLaunchdService("unload", system)
	case "windows":
		return controlWindowsService("disable", system)
	default:
		return fmt.Errorf("service control not supported on %s", runtime.GOOS)
	}
}

func getServiceStatus(system bool) (string, error) {
	switch runtime.GOOS {
	case "linux":
		return getSystemdServiceStatus(system)
	case "darwin":
		return getLaunchdServiceStatus(system)
	case "windows":
		return getWindowsServiceStatus(system)
	default:
		return "", fmt.Errorf("service status not supported on %s", runtime.GOOS)
	}
}

// Linux systemd implementation
func installSystemdService(system, enable bool) error {
	// Get clipmand binary path
	daemonPath, err := getClipmanDaemonPath()
	if err != nil {
		return err
	}

	// Create service content
	serviceContent := generateSystemdServiceContent(daemonPath, system)

	// Determine service file path
	var servicePath string
	if system {
		servicePath = "/etc/systemd/system/clipman.service"
	} else {
		userServiceDir := filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user")
		if err := os.MkdirAll(userServiceDir, 0755); err != nil {
			return fmt.Errorf("failed to create user systemd directory: %w", err)
		}
		servicePath = filepath.Join(userServiceDir, "clipman.service")
	}

	// Write service file
	if err := os.WriteFile(servicePath, []byte(serviceContent), 0644); err != nil {
		return fmt.Errorf("failed to write service file: %w", err)
	}

	// Reload systemd
	if err := reloadSystemd(system); err != nil {
		return fmt.Errorf("failed to reload systemd: %w", err)
	}

	// Enable service if requested
	if enable {
		if err := controlSystemdService("enable", system); err != nil {
			logger := GetZapLogger()
			logger.Warn("Failed to enable service", zap.Error(err))
		}
	}

	return nil
}

func uninstallSystemdService(system bool) error {
	// Stop and disable service first
	controlSystemdService("stop", system)
	controlSystemdService("disable", system)

	// Remove service file
	var servicePath string
	if system {
		servicePath = "/etc/systemd/system/clipman.service"
	} else {
		servicePath = filepath.Join(os.Getenv("HOME"), ".config", "systemd", "user", "clipman.service")
	}

	if err := os.Remove(servicePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to remove service file: %w", err)
	}

	// Reload systemd
	return reloadSystemd(system)
}

func controlSystemdService(action string, system bool) error {
	var cmd *exec.Cmd
	if system {
		cmd = exec.Command("systemctl", action, "clipman.service")
	} else {
		cmd = exec.Command("systemctl", "--user", action, "clipman.service")
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl %s failed: %w\nOutput: %s", action, err, string(output))
	}

	return nil
}

func getSystemdServiceStatus(system bool) (string, error) {
	var cmd *exec.Cmd
	if system {
		cmd = exec.Command("systemctl", "status", "clipman.service", "--no-pager")
	} else {
		cmd = exec.Command("systemctl", "--user", "status", "clipman.service", "--no-pager")
	}

	output, err := cmd.CombinedOutput()
	if err != nil {
		// systemctl status returns non-zero for inactive services, which is normal
		return string(output), nil
	}

	return string(output), nil
}

func reloadSystemd(system bool) error {
	var cmd *exec.Cmd
	if system {
		cmd = exec.Command("systemctl", "daemon-reload")
	} else {
		cmd = exec.Command("systemctl", "--user", "daemon-reload")
	}

	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl daemon-reload failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

func generateSystemdServiceContent(daemonPath string, system bool) string {
	var user string
	if system {
		user = "clipman"
	} else {
		user = os.Getenv("USER")
	}

	return fmt.Sprintf(`[Unit]
Description=Clipman Clipboard Manager Daemon
After=network.target
%s

[Service]
Type=simple
User=%s
Environment=DISPLAY=:0
Environment=XAUTHORITY=%s/.Xauthority
ExecStart=%s
Restart=on-failure
RestartSec=5

[Install]
WantedBy=%s
`, 
		func() string {
			if system {
				return "Wants=network-online.target"
			}
			return ""
		}(),
		user,
		func() string {
			if system {
				return "/home/" + user
			}
			return os.Getenv("HOME")
		}(),
		daemonPath,
		func() string {
			if system {
				return "multi-user.target"
			}
			return "default.target"
		}())
}

// macOS launchd implementation
func installLaunchdService(system, enable bool) error {
	// TODO: Implement launchd service installation
	return fmt.Errorf("launchd service installation not yet implemented")
}

func uninstallLaunchdService(system bool) error {
	// TODO: Implement launchd service uninstallation
	return fmt.Errorf("launchd service uninstallation not yet implemented")
}

func controlLaunchdService(action string, system bool) error {
	// TODO: Implement launchd service control
	return fmt.Errorf("launchd service control not yet implemented")
}

func getLaunchdServiceStatus(system bool) (string, error) {
	// TODO: Implement launchd service status
	return "", fmt.Errorf("launchd service status not yet implemented")
}

// Windows service implementation
func installWindowsService(system, enable bool) error {
	// TODO: Implement Windows service installation
	return fmt.Errorf("Windows service installation not yet implemented")
}

func uninstallWindowsService(system bool) error {
	// TODO: Implement Windows service uninstallation
	return fmt.Errorf("Windows service uninstallation not yet implemented")
}

func controlWindowsService(action string, system bool) error {
	// TODO: Implement Windows service control
	return fmt.Errorf("Windows service control not yet implemented")
}

func getWindowsServiceStatus(system bool) (string, error) {
	// TODO: Implement Windows service status
	return "", fmt.Errorf("Windows service status not yet implemented")
}

// Helper functions
func getClipmanDaemonPath() (string, error) {
	// First try to find clipmand in the same directory as current executable
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("failed to get executable path: %w", err)
	}

	execDir := filepath.Dir(executable)
	daemonPath := filepath.Join(execDir, "clipmand")

	// Check if clipmand exists in the same directory
	if _, err := os.Stat(daemonPath); err == nil {
		return daemonPath, nil
	}

	// Try to find clipmand in PATH
	if path, err := exec.LookPath("clipmand"); err == nil {
		return path, nil
	}

	// Try common installation paths
	commonPaths := []string{
		"/usr/local/bin/clipmand",
		"/usr/bin/clipmand",
		filepath.Join(os.Getenv("HOME"), ".local", "bin", "clipmand"),
	}

	for _, path := range commonPaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}

	return "", fmt.Errorf("clipmand binary not found. Please ensure clipmand is installed and in PATH")
} 