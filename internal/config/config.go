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

type Config struct {
	LogLevel        string        `json:"log_level"`
	BrokerURL       string        `json:"broker_url"`
	BrokerUsername  string        `json:"broker_username"`
	BrokerPassword  string        `json:"broker_password"`
	DeviceID        string        `json:"device_id"`
	PollingInterval time.Duration `json:"polling_interval"`
	DataDir         string        `json:"data_dir"`
	
	// History options for filtering and display
	History         HistoryOptions `json:"history"`
}

var DefaultConfig = Config{
	LogLevel:        "info",
	PollingInterval: 1 * time.Second,
	History: HistoryOptions{
		Limit:   0,      // No limit by default
		Reverse: false,  // Oldest first by default
	},
}

// These variables will be set by the init() functions in the platform-specific files
var getConfigPath func() (string, error)
var getDefaultDataDir func() (string, error)

// This can be overridden in tests
var generateDeviceID = defaultGenerateDeviceID

func Load() (*Config, error) {
	configPath, err := getConfigPath()
	if err != nil {
		return nil, fmt.Errorf("failed to get config path: %v", err)
	}

	config := DefaultConfig

	if _, err := os.Stat(configPath); err == nil {
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

	return &config, nil
}

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

func defaultGenerateDeviceID() string {
	return utils.GenerateUUID()
}
