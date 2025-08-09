// File: internal/config/config.go

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

// Function variables for testing
var (
	getConfigPath     = GetActiveConfigPath
	getDefaultDataDir = func() (string, error) {
		paths, err := GetConfigPaths()
		if err != nil {
			return "", err
		}
		return paths.DataDir, nil
	}
	generateDeviceID = func() string {
		return uuid.New().String()
	}
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

	// HTML processing options
	HTML HTMLProcessingConfig `json:"html_processing" yaml:"html_processing"`

	// Launch options
	LaunchAtStartup bool `json:"launch_at_startup" yaml:"launch_at_startup"`
	LaunchOnLogin   bool `json:"launch_on_login" yaml:"launch_on_login"`
}

// HTMLProcessingConfig holds HTML content processing options
type HTMLProcessingConfig struct {
	// Extract plain text from HTML content automatically
	ExtractText bool `json:"extract_text" yaml:"extract_text"`
	// When extract_text is true, prefer extracted text over raw HTML
	PreferExtractedText bool `json:"prefer_extracted_text" yaml:"prefer_extracted_text"`
	// Keep both raw HTML and extracted text when possible
	KeepBoth bool `json:"keep_both" yaml:"keep_both"`
}

// LogConfig holds logging-related configuration
type LogConfig struct {
	EnableConsoleLogging bool   `json:"enable_console_logging" yaml:"enable_console_logging"`
	Level               string `json:"level" yaml:"level"`
	EnableFileLogging bool   `json:"enable_file_logging" yaml:"enable_file_logging"`
	MaxLogSize        int    `json:"max_log_size" yaml:"max_log_size"`
	MaxLogFiles       int    `json:"max_log_files" yaml:"max_log_files"`
	Format            string `json:"format" yaml:"format"` // "json" or "text"
}

// StorageConfig holds storage-related configuration for any backend
type StorageConfig struct {
	// Storage backend selection
	Type             string `json:"type" yaml:"type"`                           // "boltdb", "sqlite", "postgres", "mongodb", "memory"
	ConnectionString string `json:"connection_string" yaml:"connection_string"` // For SQL/NoSQL databases
	
	// File-based storage (BoltDB, SQLite)
	DBPath string `json:"db_path" yaml:"db_path"`
	
	// Legacy BoltDB options (maintained for backward compatibility)
	MaxSize   int64 `json:"max_size" yaml:"max_size"`     // Cache size limit (BoltDB legacy)
	KeepItems int   `json:"keep_items" yaml:"keep_items"` // Items to keep on flush (BoltDB legacy)
	
	// Advanced storage options (all backends)
	Advanced AdvancedStorageConfig `json:"advanced" yaml:"advanced"`
	
	// Pool and connection settings (SQL/NoSQL)
	PoolConfig PoolConfig `json:"pool" yaml:"pool"`
	
	// Security settings
	Security SecurityConfig `json:"security" yaml:"security"`
}

// AdvancedStorageConfig holds advanced storage settings
type AdvancedStorageConfig struct {
	// Performance tuning
	BatchSize          int   `json:"batch_size" yaml:"batch_size"`                     // Batch operations size
	CacheSize          int64 `json:"cache_size" yaml:"cache_size"`                     // In-memory cache size
	Compression        bool  `json:"compression" yaml:"compression"`                   // Enable compression
	CompressionLevel   int   `json:"compression_level" yaml:"compression_level"`       // 1-9, higher = better compression
	
	// Data retention policies
	MaxItems           int64 `json:"max_items" yaml:"max_items"`                       // Maximum items to store (0 = unlimited)
	MaxAge             int64 `json:"max_age_hours" yaml:"max_age_hours"`               // Auto-delete items older than N hours (0 = never)
	CleanupInterval    int64 `json:"cleanup_interval_hours" yaml:"cleanup_interval_hours"` // How often to run cleanup (default: 24)
	
	// Query optimization
	Indexes            []string `json:"indexes" yaml:"indexes"`                       // Fields to create indexes on
	QueryTimeout       int      `json:"query_timeout_seconds" yaml:"query_timeout_seconds"` // Query timeout (default: 30)
	
	// Backup and recovery
	AutoBackup         bool   `json:"auto_backup" yaml:"auto_backup"`                 // Enable automatic backups
	BackupInterval     int64  `json:"backup_interval_hours" yaml:"backup_interval_hours"` // Hours between backups
	BackupRetention    int    `json:"backup_retention_days" yaml:"backup_retention_days"` // Days to keep backups
	BackupPath         string `json:"backup_path" yaml:"backup_path"`                 // Backup directory
	
	// Experimental features
	EnableStatistics   bool `json:"enable_statistics" yaml:"enable_statistics"`     // Track usage statistics
	EnableFullTextSearch bool `json:"enable_fts" yaml:"enable_fts"`                 // Full-text search capabilities
}

// PoolConfig holds database connection pool settings
type PoolConfig struct {
	MaxOpenConnections int   `json:"max_open_connections" yaml:"max_open_connections"` // Max open connections
	MaxIdleConnections int   `json:"max_idle_connections" yaml:"max_idle_connections"` // Max idle connections
	ConnMaxLifetime    int64 `json:"conn_max_lifetime_minutes" yaml:"conn_max_lifetime_minutes"` // Minutes
	ConnMaxIdleTime    int64 `json:"conn_max_idle_minutes" yaml:"conn_max_idle_minutes"` // Minutes
}

// SecurityConfig holds storage security settings
type SecurityConfig struct {
	Encryption         bool   `json:"encryption" yaml:"encryption"`                   // Enable at-rest encryption
	EncryptionKey      string `json:"encryption_key" yaml:"encryption_key"`           // Encryption key (use env var!)
	TLSEnabled         bool   `json:"tls_enabled" yaml:"tls_enabled"`                 // Enable TLS for network connections
	TLSCertPath        string `json:"tls_cert_path" yaml:"tls_cert_path"`             // TLS certificate path
	TLSKeyPath         string `json:"tls_key_path" yaml:"tls_key_path"`               // TLS key path
	TLSSkipVerify      bool   `json:"tls_skip_verify" yaml:"tls_skip_verify"`         // Skip TLS verification (dangerous!)
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
	paths, err := GetConfigPaths()
	if err != nil {
		// Fallback paths if we can't get proper ones
		homeDir, _ := os.UserHomeDir()
		if homeDir == "" {
			homeDir = "/tmp"
		}
		paths = &ConfigPaths{
			BaseDir:      filepath.Join(homeDir, ".config", "clipman"),
			ConfigsDir:   filepath.Join(homeDir, ".config", "clipman", "configs"),
			ActiveDir:    filepath.Join(homeDir, ".config", "clipman", "active"),
			CacheDir:     filepath.Join(homeDir, ".config", "clipman", "cache"),
			ActiveConfig: filepath.Join(homeDir, ".config", "clipman", "active", "config.yaml"),
			DataDir:      filepath.Join(homeDir, ".clipman"),
			DBFile:       filepath.Join(homeDir, ".clipman", "clipman.db"),
			LogDir:       filepath.Join(homeDir, ".clipman", "logs"),
			TempDir:      filepath.Join(homeDir, ".clipman", "temp"),
		}
	}

	// Get hostname
	hostname, err := os.Hostname()
	if err != nil {
		hostname = "unknown"
	}

	// Get platform-specific defaults
	platformDefaults := GetPlatformDefaults()

	cfg := &Config{
		DeviceID:      uuid.New().String(),
		DeviceName:    hostname,
		EnableLogging: true,
		SystemPaths:   *paths,
		Log: LogConfig{
			EnableConsoleLogging: true,
			Level:               "info",
			EnableFileLogging:   true,
			MaxLogSize:          platformDefaults.MaxLogSize,
			MaxLogFiles:         platformDefaults.MaxLogFiles,
			Format:              "text",
		},
		History: HistoryOptions{
			Limit:   0, // No limit
			Reverse: true,
		},
		Storage: DefaultStorageConfig(paths),
		Server: ServerConfig{
			Port: 8080,
			Host: "localhost",
			Path: "/api/v1",
		},
		Sync: types.SyncConfig{
			Enabled:             false, // Disabled by default for new users
			SyncOverInternet:    false,
			UseRelayNodes:       true,
			ListenPort:          0, // Dynamic port
			DiscoveryMethod:     "paired", // Changed from "mdns" to "paired" for better security
			PairingEnabled:      true,
			DeviceName:          hostname,
			AllowOnlyKnownPeers: true,
		},
		StealthMode:     platformDefaults.StealthMode,
		PollingInterval: platformDefaults.PollingInterval,
		HTML: HTMLProcessingConfig{
			ExtractText:         false, // Disabled by default
			PreferExtractedText: true,  // When enabled, prefer extracted text
			KeepBoth:            false, // Don't keep both by default
		},
		LaunchAtStartup: platformDefaults.LaunchAtStartup,
		LaunchOnLogin:   false,
	}

	return cfg
}

// EnsureDefaultConfig ensures that the default configuration is set up
func EnsureDefaultConfig() error {
	paths, err := GetConfigPaths()
	if err != nil {
		return fmt.Errorf("failed to get config paths: %w", err)
	}

	// Create default config
	cfg := DefaultConfig()

	// Save to active config
	if err := cfg.Save(paths.ActiveConfig); err != nil {
		return fmt.Errorf("failed to save default config: %w", err)
	}

	// Save a copy to configs directory as default.yaml
	defaultConfigPath := filepath.Join(paths.ConfigsDir, "default.yaml")
	if err := cfg.Save(defaultConfigPath); err != nil {
		return fmt.Errorf("failed to save default config template: %w", err)
	}

	return nil
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
			// Ensure default config is set up
			if err := EnsureDefaultConfig(); err != nil {
				return nil, fmt.Errorf("failed to set up default config: %w", err)
			}
			// Try loading again
			return Load(configPath)
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

	// Set default storage type if missing
	if cfg.Storage.Type == "" {
		cfg.Storage.Type = "boltdb"
	}

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
	
	// Log settings
	if val := os.Getenv("CLIPMAN_LOG_LEVEL"); val != "" {
		config.Log.Level = val
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
	
	// Storage settings
	if val := os.Getenv("CLIPMAN_STORAGE_TYPE"); val != "" {
		config.Storage.Type = val
	}
	if val := os.Getenv("CLIPMAN_DB_PATH"); val != "" {
		config.Storage.DBPath = val
	}
	if val := os.Getenv("CLIPMAN_CONNECTION_STRING"); val != "" {
		config.Storage.ConnectionString = val
	}
	if val := os.Getenv("CLIPMAN_STORAGE_MAX_SIZE"); val != "" {
		if size, err := strconv.ParseInt(val, 10, 64); err == nil {
			config.Storage.MaxSize = size
		}
	}
	if val := os.Getenv("CLIPMAN_STORAGE_KEEP_ITEMS"); val != "" {
		if items, err := strconv.Atoi(val); err == nil {
			config.Storage.KeepItems = items
		}
	}
	
	// Advanced storage settings
	if val := os.Getenv("CLIPMAN_STORAGE_COMPRESSION"); val != "" {
		config.Storage.Advanced.Compression = val == "true"
	}
	if val := os.Getenv("CLIPMAN_STORAGE_MAX_ITEMS"); val != "" {
		if items, err := strconv.ParseInt(val, 10, 64); err == nil {
			config.Storage.Advanced.MaxItems = items
		}
	}
	if val := os.Getenv("CLIPMAN_STORAGE_MAX_AGE_HOURS"); val != "" {
		if hours, err := strconv.ParseInt(val, 10, 64); err == nil {
			config.Storage.Advanced.MaxAge = hours
		}
	}
	if val := os.Getenv("CLIPMAN_STORAGE_AUTO_BACKUP"); val != "" {
		config.Storage.Advanced.AutoBackup = val == "true"
	}
	if val := os.Getenv("CLIPMAN_STORAGE_BACKUP_PATH"); val != "" {
		config.Storage.Advanced.BackupPath = val
	}
	
	// Security settings
	if val := os.Getenv("CLIPMAN_STORAGE_ENCRYPTION"); val != "" {
		config.Storage.Security.Encryption = val == "true"
	}
	if val := os.Getenv("CLIPMAN_STORAGE_ENCRYPTION_KEY"); val != "" {
		config.Storage.Security.EncryptionKey = val
	}
	if val := os.Getenv("CLIPMAN_STORAGE_TLS"); val != "" {
		config.Storage.Security.TLSEnabled = val == "true"
	}
}

// DefaultStorageConfig creates a default storage configuration
func DefaultStorageConfig(paths *ConfigPaths) StorageConfig {
	platformDefaults := GetPlatformDefaults()
	
	return StorageConfig{
		// Default to BoltDB for backward compatibility
		Type:   "boltdb",
		DBPath: paths.DBFile,
		
		// Legacy BoltDB settings (maintained for backward compatibility)
		MaxSize:   platformDefaults.MaxContentSize,
		KeepItems: 50,
		
		// Advanced settings with sensible defaults
		Advanced: AdvancedStorageConfig{
			// Performance tuning
			BatchSize:        100,  // Batch operations
			CacheSize:        platformDefaults.CacheSize,
			Compression:      true, // Enable compression by default
			CompressionLevel: 6,    // Balanced compression
			
			// Data retention (disabled by default)
			MaxItems:        0,  // Unlimited items
			MaxAge:          0,  // Never auto-delete
			CleanupInterval: 24, // Daily cleanup check
			
			// Query optimization
			Indexes:      []string{"type", "device_id", "created"}, // Common indexes
			QueryTimeout: 30, // 30 second timeout
			
			// Backup (disabled by default)
			AutoBackup:      false,
			BackupInterval:  24 * 7, // Weekly backups
			BackupRetention: 30,     // Keep 30 days
			BackupPath:      filepath.Join(paths.DataDir, "backups"),
			
			// Experimental features (disabled by default)
			EnableStatistics:    true,  // Enable statistics
			EnableFullTextSearch: false, // Disable FTS by default
		},
		
		// Connection pool (for SQL/NoSQL databases)
		PoolConfig: PoolConfig{
			MaxOpenConnections: 10,  // Conservative
			MaxIdleConnections: 2,   // Keep few idle
			ConnMaxLifetime:    60,  // 60 minutes
			ConnMaxIdleTime:    10,  // 10 minutes idle
		},
		
		// Security (minimal by default)
		Security: SecurityConfig{
			Encryption:    false, // Disabled by default
			TLSEnabled:    false, // Disabled by default
			TLSSkipVerify: false, // Safe default
		},
	}
}

// CreateStorageFromConfig creates a storage instance from config using the factory pattern
func CreateStorageFromConfig(cfg *Config, logger interface{}) (interface{}, error) {
	// This would typically be imported from storage package, but to avoid circular imports,
	// we'll return the configuration that the caller can use with the storage factory
	return &StorageFactoryParams{
		Type:             cfg.Storage.Type,
		DBPath:           cfg.Storage.DBPath,
		ConnectionString: cfg.Storage.ConnectionString,
		DeviceID:         cfg.DeviceID,
		Logger:           logger,
		Advanced:         cfg.Storage.Advanced,
		Pool:             cfg.Storage.PoolConfig,
		Security:         cfg.Storage.Security,
		LegacyMaxSize:    cfg.Storage.MaxSize,
		LegacyKeepItems:  cfg.Storage.KeepItems,
	}, nil
}

// StorageFactoryParams holds all parameters needed for the storage factory
type StorageFactoryParams struct {
	Type             string
	DBPath           string
	ConnectionString string
	DeviceID         string
	Logger           interface{}
	Advanced         AdvancedStorageConfig
	Pool             PoolConfig
	Security         SecurityConfig
	LegacyMaxSize    int64
	LegacyKeepItems  int
}

// GetPaths returns the system paths for this config
func (c *Config) GetPaths() ConfigPaths {
	return c.SystemPaths
}
