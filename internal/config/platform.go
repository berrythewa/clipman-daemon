// File: internal/config/platform.go

package config

import (
	"runtime"
)

// PlatformDefaults holds platform-specific default values
type PlatformDefaults struct {
	// Clipboard monitoring
	PollingInterval    int64 `json:"polling_interval" yaml:"polling_interval"`       // milliseconds
	MaxPollingInterval int64 `json:"max_polling_interval" yaml:"max_polling_interval"` // milliseconds
	StealthMode        bool  `json:"stealth_mode" yaml:"stealth_mode"`
	
	// Content handling
	MaxContentSize int64    `json:"max_content_size" yaml:"max_content_size"` // bytes
	ContentTypes   []string `json:"content_types" yaml:"content_types"`
	
	// System integration
	LaunchAtStartup      bool   `json:"launch_at_startup" yaml:"launch_at_startup"`
	TrayEnabled          bool   `json:"tray_enabled" yaml:"tray_enabled"`
	NotificationsEnabled bool   `json:"notifications_enabled" yaml:"notifications_enabled"`
	ServiceName          string `json:"service_name" yaml:"service_name"`
	
	// Performance
	CacheSize        int64 `json:"cache_size" yaml:"cache_size"`           // bytes
	MaxLogSize       int   `json:"max_log_size" yaml:"max_log_size"`       // bytes
	MaxLogFiles      int   `json:"max_log_files" yaml:"max_log_files"`
	
	// Security
	FilePermissions  int    `json:"file_permissions" yaml:"file_permissions"`   // octal
	KeyringBackend   string `json:"keyring_backend" yaml:"keyring_backend"`
	RequireAuth      bool   `json:"require_auth" yaml:"require_auth"`
}

// GetPlatformDefaults returns platform-optimized default values
func GetPlatformDefaults() PlatformDefaults {
	switch runtime.GOOS {
	case "windows":
		return PlatformDefaults{
			// Windows has efficient event-based clipboard monitoring
			PollingInterval:    100,  // 100ms - less polling needed
			MaxPollingInterval: 500,  // 500ms max
			StealthMode:        true, // Windows users prefer minimal notifications
			
			// Windows supports rich content types
			MaxContentSize: 50 * 1024 * 1024, // 50MB - Windows can handle larger content
			ContentTypes:   []string{"text", "image", "file", "url", "html", "rtf"},
			
			// Windows system integration
			LaunchAtStartup:      false, // User choice
			TrayEnabled:          true,  // Windows has good tray support
			NotificationsEnabled: true,  // Windows 10+ notifications
			ServiceName:          "Clipman Service",
			
			// Windows performance
			CacheSize:   100 * 1024 * 1024, // 100MB cache
			MaxLogSize:  20 * 1024 * 1024,  // 20MB logs
			MaxLogFiles: 10,
			
			// Windows security
			FilePermissions: 0644, // Not as relevant on Windows
			KeyringBackend:  "windows-credential-manager",
			RequireAuth:     false,
		}
		
	case "darwin":
		return PlatformDefaults{
			// macOS uses change count detection
			PollingInterval:    500,  // 500ms - change count based
			MaxPollingInterval: 1000, // 1s max
			StealthMode:        false, // macOS users expect some feedback
			
			// macOS supports rich pasteboard types
			MaxContentSize: 30 * 1024 * 1024, // 30MB - reasonable for macOS
			ContentTypes:   []string{"text", "image", "file", "url", "html", "rtf"},
			
			// macOS system integration
			LaunchAtStartup:      false, // User choice
			TrayEnabled:          false, // macOS uses menu bar
			NotificationsEnabled: true,  // macOS notification center
			ServiceName:          "com.berrythewa.clipman",
			
			// macOS performance
			CacheSize:   75 * 1024 * 1024,  // 75MB cache
			MaxLogSize:  15 * 1024 * 1024,  // 15MB logs
			MaxLogFiles: 7,
			
			// macOS security
			FilePermissions: 0755, // Unix permissions
			KeyringBackend:  "macos-keychain",
			RequireAuth:     false,
		}
		
	default: // Linux and other Unix-like systems
		return PlatformDefaults{
			// Linux requires more frequent polling
			PollingInterval:    500,  // 500ms - X11/Wayland polling
			MaxPollingInterval: 2000, // 2s max to balance performance
			StealthMode:        false, // Linux users often prefer transparency
			
			// Linux content handling varies by DE
			MaxContentSize: 20 * 1024 * 1024, // 20MB - conservative for various DEs
			ContentTypes:   []string{"text", "image", "file", "url", "html"},
			
			// Linux system integration
			LaunchAtStartup:      false, // User choice
			TrayEnabled:          true,  // Most DEs support tray
			NotificationsEnabled: true,  // Most DEs have notifications
			ServiceName:          "clipman",
			
			// Linux performance
			CacheSize:   50 * 1024 * 1024,  // 50MB cache - conservative
			MaxLogSize:  10 * 1024 * 1024,  // 10MB logs
			MaxLogFiles: 5,
			
			// Linux security
			FilePermissions: 0755, // Unix permissions
			KeyringBackend:  "linux-keyring",
			RequireAuth:     false,
		}
	}
}

// ApplyPlatformDefaults updates a config with platform-specific defaults
func ApplyPlatformDefaults(cfg *Config) {
	defaults := GetPlatformDefaults()
	
	// Apply platform defaults if not explicitly set
	if cfg.PollingInterval == 0 {
		cfg.PollingInterval = defaults.PollingInterval
	}
	
	// Update log config with platform defaults
	if cfg.Log.MaxLogSize == 0 {
		cfg.Log.MaxLogSize = defaults.MaxLogSize
	}
	
	if cfg.Log.MaxLogFiles == 0 {
		cfg.Log.MaxLogFiles = defaults.MaxLogFiles
	}
	
	// Update storage config with platform defaults
	if cfg.Storage.MaxSize == 0 {
		cfg.Storage.MaxSize = defaults.MaxContentSize
	}
}

// GetPlatformSpecificPaths returns paths that are platform-aware with proper expansion
func GetPlatformSpecificPaths() (*ConfigPaths, error) {
	return GetConfigPaths() // This already handles platform differences
}

// ValidatePlatformConfig validates platform-specific configuration
func ValidatePlatformConfig(cfg *Config) error {
	// Add platform-specific validation logic here
	// For example, check if tray is requested on a platform that doesn't support it
	return nil
} 