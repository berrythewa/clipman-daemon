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
)

// SystemPaths holds all the important file and directory paths used by the application
type SystemPaths struct {
	// Config file location
	ConfigFile string
	
	// Base directory for all application data
	DataDir string
	
	// Database file location
	DBFile string
	
	// Directory for temporary files
	TempDir string
	
	// Directory for log files
	LogDir string
}

// HistoryOptions defines options for retrieving and displaying clipboard history
type HistoryOptions struct {
	// Maximum number of entries to return (0 means no limit)
	Limit int `json:"limit"`
	
	// Return entries starting from this time (zero value means no time filter)
	Since time.Time `json:"since"`
	
	// Return entries before this time (zero value means no time filter)
	Before time.Time `json:"before"`
	
	// Filter by content type (empty means all types)
	ContentType types.ContentType `json:"content_type"`
	
	// Reverse order (newest first when true, oldest first when false)
	Reverse bool `json:"reverse"`
	
	// Minimum size in bytes (0 means no minimum)
	MinSize int `json:"min_size"`
	
	// Maximum size in bytes (0 means no maximum)
	MaxSize int `json:"max_size"`
}

// StorageConfig defines storage-related configuration
type StorageConfig struct {
	// Path to the database file (if empty, default path will be used)
	DBPath string `json:"db_path"`
	
	// Maximum size of the clipboard history cache in bytes
	MaxSize int64 `json:"max_size"`
	
	// Number of items to keep when flushing the cache
	KeepItems int `json:"keep_items"`
}

// BrokerConfig defines message broker configuration
type BrokerConfig struct {
	// URL of the message broker
	URL string `json:"url"`
	
	// Username for broker authentication
	Username string `json:"username"`
	
	// Password for broker authentication
	Password string `json:"password"`
}

// Config is the main configuration struct
type Config struct {
	// General settings
	LogLevel        string        `json:"log_level"`
	DeviceID        string        `json:"device_id"`
	PollingInterval time.Duration `json:"polling_interval"`
	DataDir         string        `json:"data_dir"`
	
	// Subsystem configurations
	Storage         StorageConfig  `json:"storage"`
	Broker          BrokerConfig   `json:"broker"`
	History         HistoryOptions `json:"history"`
}

// DefaultConfig provides sensible defaults for the application
var DefaultConfig = Config{
	LogLevel:        "info",
	PollingInterval: 1 * time.Second,
	Storage: StorageConfig{
		MaxSize:   100 * 1024 * 1024, // 100MB default
		KeepItems: 10,                // Keep 10 items when flushing
	},
	History: HistoryOptions{
		Limit:   0,     // No limit by default
		Reverse: false, // Oldest first by default
	},
}

// These variables will be set by the init() functions in the platform-specific files
var getConfigPath func() (string, error)
var getDefaultDataDir func() (string, error)

// This can be overridden in tests
var generateDeviceID = defaultGenerateDeviceID

// GetPaths returns all important file system paths used by the application
func (c *Config) GetPaths() SystemPaths {
	// Get the database path
	dbPath := c.Storage.DBPath
	if dbPath == "" {
		dbPath = filepath.Join(c.DataDir, "clipboard.db")
	}
	
	// Get config path
	configPath, _ := getConfigPath()
	
	// Set up log directory
	logDir := filepath.Join(c.DataDir, "logs")
	
	// Set up temp directory
	tempDir := filepath.Join(c.DataDir, "temp")
	
	return SystemPaths{
		ConfigFile: configPath,
		DataDir:    c.DataDir,
		DBFile:     dbPath,
		LogDir:     logDir,
		TempDir:    tempDir,
	}
}

// Load loads the configuration from the file system or environment
func Load() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get config path: %v", err)
	}

	config := DefaultConfig
	configExists := false

	// Check if config file exists
	if _, err := os.Stat(configPath); err == nil {
		configExists = true
		file, err := os.Open(configPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open config file: %v", err)
		}
		defer file.Close()

		if err := json.NewDecoder(file).Decode(&config); err != nil {
			return nil, fmt.Errorf("failed to decode config file: %v", err)
		}
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("error checking config file: %v", err)
	}

	// Override from environment variables
	config = overrideFromEnv(config)

	// Set computed values
	if config.DeviceID == "" {
		config.DeviceID = generateDeviceID()
	}
	if config.DataDir == "" {
		config.DataDir, err = getDefaultDataDir()
		if err != nil {
			return nil, fmt.Errorf("failed to get default data directory: %v", err)
		}
	}

	// Ensure the data directory exists
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %v", err)
	}

	// Save default config if it doesn't exist
	if !configExists {
		// Create log directories
		paths := config.GetPaths()
		if err := os.MkdirAll(paths.LogDir, 0755); err != nil {
			// Just log this error, don't fail
			fmt.Printf("Warning: failed to create log directory: %v\n", err)
		}
		
		// Create temp directory
		if err := os.MkdirAll(paths.TempDir, 0755); err != nil {
			// Just log this error, don't fail
			fmt.Printf("Warning: failed to create temp directory: %v\n", err)
		}

		// Save the config
		if err := config.Save(); err != nil {
			// Just log this error, don't fail
			fmt.Printf("Warning: failed to save default config: %v\n", err)
		}
	}

	return &config, nil
}

// overrideFromEnv checks for environment variables and overrides config values
func overrideFromEnv(config Config) Config {
	// General settings
	if val := os.Getenv("CLIPMAN_LOG_LEVEL"); val != "" {
		config.LogLevel = val
	}
	if val := os.Getenv("CLIPMAN_DEVICE_ID"); val != "" {
		config.DeviceID = val
	}
	if val := os.Getenv("CLIPMAN_DATA_DIR"); val != "" {
		config.DataDir = val
	}
	
	// Broker settings
	if val := os.Getenv("CLIPMAN_BROKER_URL"); val != "" {
		config.Broker.URL = val
	}
	if val := os.Getenv("CLIPMAN_BROKER_USERNAME"); val != "" {
		config.Broker.Username = val
	}
	if val := os.Getenv("CLIPMAN_BROKER_PASSWORD"); val != "" {
		config.Broker.Password = val
	}
	
	// Storage settings
	if val := os.Getenv("CLIPMAN_STORAGE_PATH"); val != "" {
		config.Storage.DBPath = val
	}
	
	return config
}

// Save saves the current configuration to a file
func (c *Config) Save() error {
	configPath, err := getConfigPath()
	if err != nil {
		return fmt.Errorf("failed to get config path: %v", err)
	}

	// Ensure the directory exists
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %v", err)
	}

	file, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("failed to create config file: %v", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(c); err != nil {
		return fmt.Errorf("failed to encode config: %v", err)
	}

	return nil
}

// defaultGenerateDeviceID generates a new UUID for device identification
func defaultGenerateDeviceID() string {
	return utils.GenerateUUID()
}
