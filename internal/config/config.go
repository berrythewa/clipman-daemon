// File: internal/config/config.go

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/berrythewa/clipman-daemon/pkg/utils"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// ConfigPaths holds all relevant paths for the application
type ConfigPaths struct {
	BaseDir      string // Base directory for all config files
	ConfigsDir   string // Directory containing config templates
	ActiveDir    string // Directory containing active configuration
	CacheDir     string // Directory for runtime cache
	ActiveConfig string // Path to active config file
	DataDir      string // Directory for application data
	DBFile       string // Path to database file
	LogDir       string // Directory for log files
	TempDir      string // Directory for temporary files
}

// Config holds all application configuration
type Config struct {
	// General settings
	DeviceID      string `json:"device_id" yaml:"device_id"`
	DeviceName    string `json:"device_name" yaml:"device_name"`
	EnableLogging bool   `json:"enable_logging" yaml:"enable_logging"`
	
	// System paths configuration
	SystemPaths ConfigPaths `json:"system_paths" yaml:"system_paths"`
	
	// Logging configuration
	Log LogConfig `json:"log" yaml:"log"`
	
	// History retrieval options
	History HistoryOptions `json:"history" yaml:"history"`
	
	// Storage configuration
	Storage StorageConfig `json:"storage" yaml:"storage"`
	
	// Server configuration
	Server ServerConfig `json:"server" yaml:"server"`
	
	// Synchronization configuration
	Sync types.SyncConfig `json:"sync" yaml:"sync"`
	
	// Clipboard monitoring options
	StealthMode     bool  `json:"stealth_mode" yaml:"stealth_mode"`
	PollingInterval int64 `json:"polling_interval" yaml:"polling_interval"`

	// Launch options
	LaunchAtStartup bool `json:"launch_at_startup" yaml:"launch_at_startup"`
	LaunchOnLogin   bool `json:"launch_on_login" yaml:"launch_on_login"`
}

// LogConfig holds logging-related configuration
type LogConfig struct {
	EnableFileLogging bool   `json:"enable_file_logging" yaml:"enable_file_logging"`
	MaxLogSize        int    `json:"max_log_size" yaml:"max_log_size"`
	MaxLogFiles       int    `json:"max_log_files" yaml:"max_log_files"`
	Format            string `json:"format" yaml:"format"` // "json" or "text"
}

// StorageConfig holds storage-related configuration
type StorageConfig struct {
	DBPath    string `json:"db_path" yaml:"db_path"`
	MaxSize   int64  `json:"max_size" yaml:"max_size"`
	KeepItems int    `json:"keep_items" yaml:"keep_items"`
}

// HistoryOptions defines options for retrieving clipboard history
type HistoryOptions struct {
	Limit       int64     `json:"limit" yaml:"limit"`
	Since       time.Time `json:"since" yaml:"since"`
	Before      time.Time `json:"before" yaml:"before"`
	ContentType types.ContentType `json:"content_type" yaml:"content_type"`
	Reverse     bool      `json:"reverse" yaml:"reverse"`
	MinSize     int64     `json:"min_size" yaml:"min_size"`
	MaxSize     int64     `json:"max_size" yaml:"max_size"`
}

// ServerConfig holds configuration for the server
type ServerConfig struct {
	Port     int64  `json:"port" yaml:"port"`
	Host     string `json:"host" yaml:"host"`
	Path     string `json:"path" yaml:"path"`
	Username string `json:"username" yaml:"username"`
}

// GetConfigPaths returns the platform-specific configuration paths
func GetConfigPaths() (*ConfigPaths, error) {
	// First check environment variable for base directory
	baseDir := os.Getenv("CLIPMAN_CONFIG_DIR")
	if baseDir == "" {
		// Get platform-specific config directory
		configDir, err := os.UserConfigDir()
		if err != nil {
			return nil, err
		}

		// Use different paths for different OSes
		switch runtime.GOOS {
		case "windows":
			baseDir = filepath.Join(configDir, "Clipman")
		case "darwin":
			baseDir = filepath.Join(configDir, "com.berrythewa.clipman")
		default: // Linux and others
			baseDir = filepath.Join(configDir, "clipman")
		}
	}

	// Get data directory
	dataDir := os.Getenv("CLIPMAN_DATA_DIR")
	if dataDir == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}

		switch runtime.GOOS {
		case "windows":
			appData, err := os.UserConfigDir()
			if err == nil {
				dataDir = filepath.Join(appData, "Clipman", "Data")
			} else {
				dataDir = filepath.Join(homeDir, "AppData", "Local", "Clipman")
			}
		case "darwin":
			libraryDir := filepath.Join(homeDir, "Library")
			dataDir = filepath.Join(libraryDir, "Application Support", "Clipman")
		default: // Linux and others
			if xdgDataHome := os.Getenv("XDG_DATA_HOME"); xdgDataHome != "" {
				dataDir = filepath.Join(xdgDataHome, "clipman")
			} else {
				dataDir = filepath.Join(homeDir, ".clipman")
			}
		}
	}

	paths := &ConfigPaths{
		BaseDir:      baseDir,
		ConfigsDir:   filepath.Join(baseDir, "configs"),
		ActiveDir:    filepath.Join(baseDir, "active"),
		CacheDir:     filepath.Join(baseDir, "cache"),
		ActiveConfig: filepath.Join(baseDir, "active", "config.yaml"),
		DataDir:      dataDir,
		DBFile:       filepath.Join(dataDir, "clipman.db"),
		LogDir:       filepath.Join(dataDir, "logs"),
		TempDir:      filepath.Join(dataDir, "temp"),
	}

	// Create directories if they don't exist
	for _, dir := range []string{
		paths.ConfigsDir,
		paths.ActiveDir,
		paths.CacheDir,
		paths.DataDir,
		paths.LogDir,
		paths.TempDir,
	} {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return nil, err
		}
	}

	return paths, nil
}

// DefaultConfig returns a new Config with default values
func DefaultConfig() *Config {
	paths, _ := GetConfigPaths() // Ignore error, will use fallback paths

	return &Config{
		DeviceID:      uuid.New().String(),
		DeviceName:    utils.GetHostname(),
		EnableLogging: true,
		SystemPaths:   *paths,
		Log: LogConfig{
			EnableFileLogging: true,
			MaxLogSize:        10 * 1024 * 1024, // 10MB
			MaxLogFiles:       5,
			Format:           "text",
		},
		History: HistoryOptions{
			Limit:   0, // No limit
			Reverse: true,
		},
		Storage: StorageConfig{
			DBPath:    paths.DBFile,
			MaxSize:   100 * 1024 * 1024, // 100MB
			KeepItems: 50,
		},
		Server: ServerConfig{
			Port: 8080,
			Host: "localhost",
			Path: "/api/v1",
		},
		Sync: types.SyncConfig{
			Enabled:           true,
			SyncOverInternet:  false,
			UseRelayNodes:     true,
			ListenPort:        0, // Dynamic port
			DiscoveryMethod:   "mdns",
			PairingEnabled:    true,
			PairingTimeout:    300,
			DeviceName:        utils.GetHostname(),
			DeviceType:        "desktop",
			AllowOnlyKnownPeers: true,
		},
		StealthMode:     true,
		PollingInterval: 10000, // 10 seconds
		LaunchAtStartup: false,
		LaunchOnLogin:   false,
	}
}

// Load loads the configuration from the specified file or creates default if not exists
func Load(configPath string) (*Config, error) {
	// If no config path provided, use default
	if configPath == "" {
		var err error
		configPath, err = GetActiveConfigPath()
		if err != nil {
			return nil, err
		}
	}

	// Read the config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// Create default config if it doesn't exist
			cfg := DefaultConfig()
			if err := cfg.Save(configPath); err != nil {
				return nil, fmt.Errorf("failed to create default config: %w", err)
			}
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse the config file
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Override with environment variables
	overrideFromEnv(&cfg)

	return &cfg, nil
}

// Save saves the configuration to the specified file
func (c *Config) Save(configPath string) error {
	// Ensure the directory exists
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal the config
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write the config file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Export exports the configuration to a YAML file
func (c *Config) Export(path string) error {
	return c.Save(path)
}

// GetActiveConfigPath returns the path to the currently active config
func GetActiveConfigPath() (string, error) {
	paths, err := GetConfigPaths()
	if err != nil {
		return "", err
	}
	return paths.ActiveConfig, nil
}

// ListAvailableConfigs returns a list of available config templates
func ListAvailableConfigs() ([]string, error) {
	paths, err := GetConfigPaths()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(paths.ConfigsDir)
	if err != nil {
		return nil, err
	}

	var configs []string
	for _, entry := range entries {
		if !entry.IsDir() && filepath.Ext(entry.Name()) == ".yaml" {
			configs = append(configs, entry.Name())
		}
	}

	return configs, nil
}

// SetActiveConfig sets the active configuration to use
func SetActiveConfig(configName string) error {
	paths, err := GetConfigPaths()
	if err != nil {
		return err
	}

	sourcePath := filepath.Join(paths.ConfigsDir, configName)
	targetPath := paths.ActiveConfig

	// Check if source config exists
	if _, err := os.Stat(sourcePath); os.IsNotExist(err) {
		return err
	}

	// Remove existing symlink if it exists
	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return err
	}

	// Create new symlink
	return os.Symlink(sourcePath, targetPath)
}

// overrideFromEnv overrides configuration values from environment variables
func overrideFromEnv(config *Config) {
	// General settings
	if val := os.Getenv("CLIPMAN_DEVICE_ID"); val != "" {
		config.DeviceID = val
	}
	if val := os.Getenv("CLIPMAN_DEVICE_NAME"); val != "" {
		config.DeviceName = val
	}
	if val := os.Getenv("CLIPMAN_DATA_DIR"); val != "" {
		config.SystemPaths.DataDir = val
	}
	
	// Sync settings
	if val := os.Getenv("CLIPMAN_SYNC_ENABLED"); val != "" {
		config.Sync.Enabled = val == "true"
	}
	if val := os.Getenv("CLIPMAN_SYNC_INTERNET"); val != "" {
		config.Sync.SyncOverInternet = val == "true"
	}
	if val := os.Getenv("CLIPMAN_SYNC_RELAY"); val != "" {
		config.Sync.UseRelayNodes = val == "true"
	}
	if val := os.Getenv("CLIPMAN_SYNC_PORT"); val != "" {
		if port, err := strconv.Atoi(val); err == nil {
			config.Sync.ListenPort = port
		}
	}
	if val := os.Getenv("CLIPMAN_SYNC_DISCOVERY"); val != "" {
		config.Sync.DiscoveryMethod = val
	}
	
	// Clipboard monitoring options
	if val := os.Getenv("CLIPMAN_STEALTH_MODE"); val != "" {
		config.StealthMode = val == "true"
	}
	if val := os.Getenv("CLIPMAN_POLLING_INTERVAL"); val != "" {
		if ms, err := strconv.ParseInt(val, 10, 64); err == nil {
			config.PollingInterval = ms
		}
	}
}
