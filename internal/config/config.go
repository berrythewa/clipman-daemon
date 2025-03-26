// File: internal/config/config.go

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/berrythewa/clipman-daemon/pkg/utils"
	"github.com/google/uuid"
)

// SystemPaths holds paths for various configuration and data files
type SystemPaths struct {
	ConfigFile string `json:"config_file"`
	DataDir    string `json:"data_dir"`
	DBFile     string `json:"db_file"`
	TempDir    string `json:"temp_dir"`
	LogDir     string `json:"log_dir"`
}

// LogConfig holds logging-related configuration
type LogConfig struct {
	EnableFileLogging bool   `json:"enable_file_logging"`
	MaxLogSize        int    `json:"max_log_size"`
	MaxLogFiles       int    `json:"max_log_files"`
	Format            string `json:"format"` // "json" or "text"
}

// StorageConfig holds storage-related configuration
type StorageConfig struct {
	DBPath    string `json:"db_path"`
	MaxSize   int64  `json:"max_size"`
	KeepItems int    `json:"keep_items"`
}

// HistoryOptions defines options for retrieving clipboard history
type HistoryOptions struct {
	Limit       int       `json:"limit"`
	Since       time.Time `json:"since"`
	Before      time.Time `json:"before"`
	ContentType string    `json:"content_type"`
	Reverse     bool      `json:"reverse"`
	MinSize     int64     `json:"min_size"`
	MaxSize     int64     `json:"max_size"`
}

// BrokerConfig holds configuration for message brokers (legacy)
type BrokerConfig struct {
	URL      string `json:"url"`
	Username string `json:"username"`
	Password string `json:"password"`
}

// Constants for synchronization modes
const (
	SyncModeP2P         = "p2p"
	SyncModeCentralized = "centralized"
)

// SyncConfig contains all synchronization settings
type SyncConfig struct {
	// Sync mode: p2p or centralized
	Mode string `json:"mode"`
	
	// MQTT broker URL for centralized mode
	URL string `json:"url"`
	
	// Optional credentials for MQTT broker
	Username string `json:"username"`
	Password string `json:"password"`
	
	// Group settings
	DefaultGroup   string   `json:"default_group"`
	AutoJoinGroups bool     `json:"auto_join_groups"`
	
	// Content filtering options
	MaxSyncSize     int64    `json:"max_sync_size"`
	AllowedTypes    []string `json:"allowed_types"`
	ExcludedTypes   []string `json:"excluded_types"`
	IncludePatterns []string `json:"include_patterns"`
	ExcludePatterns []string `json:"exclude_patterns"`
	
	// Security settings
	EnableEncryption bool   `json:"enable_encryption"`
	EncryptionKey    string `json:"encryption_key,omitempty"`
}

// Config holds all application configuration
type Config struct {
	// General settings
	DeviceID      string `json:"device_id"`
	DeviceName    string `json:"device_name"`
	EnableLogging bool   `json:"enable_logging"`
	
	// System paths configuration
	SystemPaths SystemPaths `json:"system_paths"`
	
	// Logging configuration
	Log LogConfig `json:"log"`
	
	// History retrieval options
	History HistoryOptions `json:"history"`
	
	// Storage configuration
	Storage StorageConfig `json:"storage"`
	
	// Legacy broker configuration, kept for compatibility
	Broker BrokerConfig `json:"broker"`
	
	// Synchronization configuration
	Sync SyncConfig `json:"sync"`
}

// DefaultSystemPaths returns default system paths
func DefaultSystemPaths() SystemPaths {
	// Get default data directory
	dataDir, err := getDefaultDataDir()
	if err != nil {
		dataDir = filepath.Join(os.TempDir(), "clipman")
	}
	
	// Config file is in $HOME/.clipman/config.json
	configFile := filepath.Join(dataDir, "config.json")
	
	// Database file is in $HOME/.clipman/clipman.db
	dbFile := filepath.Join(dataDir, "clipman.db")
	
	// Log directory is in $HOME/.clipman/logs
	logDir := filepath.Join(dataDir, "logs")
	
	// Temp directory is in $HOME/.clipman/temp
	tempDir := filepath.Join(dataDir, "temp")
	
	return SystemPaths{
		ConfigFile: configFile,
		DataDir:    dataDir,
		DBFile:     dbFile,
		LogDir:     logDir,
		TempDir:    tempDir,
	}
}

// DefaultLogConfig returns default logging configuration
func DefaultLogConfig() LogConfig {
	return LogConfig{
		EnableFileLogging: true,
		MaxLogSize:        10 * 1024 * 1024, // 10MB
		MaxLogFiles:       5,                // Keep 5 log files
		Format:            "text",           // Default to plain text format
	}
}

// DefaultStorageConfig returns default storage configuration
func DefaultStorageConfig() StorageConfig {
	return StorageConfig{
		DBPath:    "",                   // Will be computed from SystemPaths
		MaxSize:   100 * 1024 * 1024,    // 100MB default
		KeepItems: 10,                   // Keep 10 items when flushing
	}
}

// DefaultHistoryOptions returns default history options
func DefaultHistoryOptions() HistoryOptions {
	return HistoryOptions{
		Limit:       50,    // Default to showing 50 items
		Reverse:     true,  // Newest first by default
		ContentType: "",    // Any content type
		MinSize:     0,     // No minimum size
		MaxSize:     0,     // No maximum size
	}
}

// DefaultBrokerConfig returns default broker configuration (legacy)
func DefaultBrokerConfig() BrokerConfig {
	return BrokerConfig{
		URL:      "",
		Username: "",
		Password: "",
	}
}

// DefaultConfig returns a configuration with sensible defaults
func DefaultConfig() *Config {
	config := &Config{
		DeviceID:      uuid.New().String(),
		DeviceName:    getDefaultDeviceName(),
		EnableLogging: true,
		SystemPaths:   DefaultSystemPaths(),
		Log:           DefaultLogConfig(),
		History:       DefaultHistoryOptions(),
		Storage:       DefaultStorageConfig(),
		Broker:        DefaultBrokerConfig(),
		Sync:          DefaultSyncConfig(),
	}
	return config
}

// GetSystemPaths returns computed system paths for the configuration
func (c *Config) GetSystemPaths() SystemPaths {
	// If a config file was specified, use its directory as the base for other paths
	configPath := c.SystemPaths.ConfigFile
	
	// Set up data directory if not specified
	dataDir := c.SystemPaths.DataDir
	if dataDir == "" {
		var err error
		dataDir, err = getDefaultDataDir()
		if err != nil {
			dataDir = filepath.Join(os.TempDir(), "clipman")
		}
	}
	
	// Set up DB path if not specified
	dbPath := c.Storage.DBPath
	if dbPath == "" {
		dbPath = filepath.Join(dataDir, "clipman.db")
	}
	
	// Set up log directory
	logDir := filepath.Join(dataDir, "logs")
	
	// Set up temp directory
	tempDir := filepath.Join(dataDir, "temp")
	
	return SystemPaths{
		ConfigFile: configPath,
		DataDir:    dataDir,
		DBFile:     dbPath,
		LogDir:     logDir,
		TempDir:    tempDir,
	}
}

// Load loads the configuration from the specified file
func Load(filePath string) (*Config, error) {
	var err error
	
	// Start with default configuration
	config := DefaultConfig()
	configExists := false
	
	// If no config file is specified, use the default location
	if filePath == "" {
		filePath = config.SystemPaths.ConfigFile
	}
	
	// Check if config file exists
	if _, err := os.Stat(filePath); err == nil {
		configExists = true
		
		// Read config from file
		file, err := os.Open(filePath)
		if err != nil {
			return nil, fmt.Errorf("failed to open config file: %v", err)
		}
		defer file.Close()
		
		if err := json.NewDecoder(file).Decode(config); err != nil {
			return nil, fmt.Errorf("failed to decode config file: %v", err)
		}
		
		// Update config file path
		config.SystemPaths.ConfigFile = filePath
	}
	
	// Override from environment variables
	overrideFromEnv(config)
	
	// Set computed values
	if config.DeviceID == "" {
		config.DeviceID = uuid.New().String()
	}
	
	if config.DeviceName == "" {
		config.DeviceName = getDefaultDeviceName()
	}
	
	// Update system paths
	config.SystemPaths = config.GetSystemPaths()
	
	// Ensure the data directory exists
	if err := os.MkdirAll(config.SystemPaths.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %v", err)
	}
	
	// If config didn't exist, create it
	if !configExists {
		if err := config.Save(); err != nil {
			return nil, fmt.Errorf("failed to save default config: %v", err)
		}
	}
	
	return config, nil
}

// Save saves the configuration to a file
func (c *Config) Save() error {
	// Ensure the directory exists
	dir := filepath.Dir(c.SystemPaths.ConfigFile)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}
	
	// Create or truncate the file
	file, err := os.Create(c.SystemPaths.ConfigFile)
	if err != nil {
		return fmt.Errorf("failed to create config file: %v", err)
	}
	defer file.Close()
	
	// Write the config
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(c); err != nil {
		return fmt.Errorf("failed to encode config: %v", err)
	}
	
	return nil
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
	if val := os.Getenv("CLIPMAN_SYNC_URL"); val != "" {
		config.Sync.URL = val
	}
	if val := os.Getenv("CLIPMAN_SYNC_USERNAME"); val != "" {
		config.Sync.Username = val
	}
	if val := os.Getenv("CLIPMAN_SYNC_PASSWORD"); val != "" {
		config.Sync.Password = val
	}
	if val := os.Getenv("CLIPMAN_SYNC_MODE"); val != "" {
		config.Sync.Mode = val
	}
	if val := os.Getenv("CLIPMAN_SYNC_GROUP"); val != "" {
		config.Sync.DefaultGroup = val
	}
}

// getDefaultDataDir returns the default data directory for the application
func getDefaultDataDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, ".clipman"), nil
}

// getDefaultDeviceName returns a default device name
func getDefaultDeviceName() string {
	hostname, err := os.Hostname()
	if err == nil && hostname != "" {
		return hostname
	}
	return utils.GenerateRandomID(8)
}
