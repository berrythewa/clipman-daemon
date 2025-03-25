package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Sync modes
const (
	SyncModeP2P         string = "p2p"
	SyncModeCentralized string = "centralized"
)

// SyncConfig defines synchronization-related configuration
type SyncConfig struct {
	// MQTT broker connection settings (compatible with legacy broker config)
	URL      string `json:"url"`       // URL of the MQTT broker
	Username string `json:"username"`  // Username for broker authentication
	Password string `json:"password"`  // Password for broker authentication

	// Sync mode
	Mode      string `json:"mode"`      // Synchronization mode: "p2p" or "centralized"

	// Group management settings
	DefaultGroup   string `json:"default_group"`   // Default synchronization group
	AutoJoinGroups bool   `json:"auto_join_groups"` // Whether to automatically join discovered groups

	// Content filtering options
	AllowedTypes    []string `json:"allowed_types"`     // Content types to allow for syncing
	ExcludedTypes   []string `json:"excluded_types"`    // Content types to exclude from syncing
	MaxSyncSize     int64    `json:"max_sync_size"`     // Maximum size in bytes for synced content
	IncludePatterns []string `json:"include_patterns"`  // Regex patterns to include
	ExcludePatterns []string `json:"exclude_patterns"`  // Regex patterns to exclude

	// Security options
	EnableEncryption bool   `json:"enable_encryption"` // Whether to enable end-to-end encryption
	EncryptionAlgo   string `json:"encryption_algo"`   // Encryption algorithm to use
	KeyRotationDays  int    `json:"key_rotation_days"` // Days between key rotations

	// Discovery options
	AnnounceInterval int `json:"announce_interval"` // Seconds between presence announcements
	PeerTimeout      int `json:"peer_timeout"`      // Seconds until a peer is considered offline
}

// DefaultSyncConfig returns a SyncConfig with sensible defaults
func DefaultSyncConfig() SyncConfig {
	return SyncConfig{
		// Default to empty broker settings - must be configured by user
		URL:      "",
		Username: "",
		Password: "", 

		// Default to P2P mode
		Mode:      SyncModeP2P,

		// Group settings
		DefaultGroup:   "", // Empty means no default group
		AutoJoinGroups: false,

		// Content filtering defaults
		AllowedTypes:    []string{}, // Empty means all types allowed
		ExcludedTypes:   []string{}, // Empty means no types excluded
		MaxSyncSize:     1024 * 1024, // 1MB default max size
		IncludePatterns: []string{},
		ExcludePatterns: []string{},

		// Security defaults
		EnableEncryption: false,
		EncryptionAlgo:   "AES-GCM",
		KeyRotationDays:  7,

		// Discovery defaults
		AnnounceInterval: 60,  // Announce presence every minute
		PeerTimeout:      300, // Consider peers offline after 5 minutes
	}
}

// Load sync configuration from a file
func LoadSyncConfig(path string) (SyncConfig, error) {
	config := DefaultSyncConfig()

	// Create the directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return config, err
	}

	// Check if the file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		// Create a new config file with defaults
		file, err := os.Create(path)
		if err != nil {
			return config, err
		}
		defer file.Close()

		encoder := json.NewEncoder(file)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(config); err != nil {
			return config, err
		}

		return config, nil
	}

	// Read the existing config file
	file, err := os.Open(path)
	if err != nil {
		return config, err
	}
	defer file.Close()

	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return config, err
	}

	return config, nil
}

// Save sync configuration to a file
func SaveSyncConfig(config SyncConfig, path string) error {
	// Create the directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Create or truncate the file
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Write the config
	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	return encoder.Encode(config)
}

// IsModeP2P returns true if the sync mode is peer-to-peer
func (c *SyncConfig) IsModeP2P() bool {
	return c.Mode == SyncModeP2P || c.Mode == ""
}

// IsModeCentralized returns true if the sync mode is centralized
func (c *SyncConfig) IsModeCentralized() bool {
	return c.Mode == SyncModeCentralized
}

// Update the main Config struct to include SyncConfig
// This will be integrated into config.go in future updates 