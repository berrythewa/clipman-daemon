package storage

import (
	"fmt"
	"strings"

	"go.uber.org/zap"
)

// StorageType represents different storage backend types
type StorageType string

const (
	StorageTypeBoltDB  StorageType = "boltdb"
	StorageTypeMemory  StorageType = "memory"
	StorageTypeSQLite  StorageType = "sqlite"
	StorageTypePostgres StorageType = "postgres" // Future implementation
	StorageTypeMongoDB  StorageType = "mongodb"  // Future implementation
)

// FactoryConfig holds configuration for creating storage instances
type FactoryConfig struct {
	Type     StorageType
	DBPath   string
	Logger   *zap.Logger
	DeviceID string
	
	// Additional configuration for different storage types
	ConnectionString string            // For SQL databases
	Options          map[string]interface{} // For any additional options
}

// StorageFactory creates storage instances based on configuration
type StorageFactory struct {
	logger *zap.Logger
}

// NewStorageFactory creates a new storage factory
func NewStorageFactory(logger *zap.Logger) *StorageFactory {
	return &StorageFactory{
		logger: logger,
	}
}

// CreateStorage creates a storage instance based on the provided configuration
func (f *StorageFactory) CreateStorage(config FactoryConfig) (IStorage, error) {
	if config.Logger == nil {
		config.Logger = f.logger
	}

	f.logger.Info("Creating storage instance", 
		zap.String("type", string(config.Type)),
		zap.String("db_path", config.DBPath),
		zap.String("device_id", config.DeviceID))

	switch strings.ToLower(string(config.Type)) {
	case string(StorageTypeBoltDB):
		return f.createBoltDBStorage(config)
	
	case string(StorageTypeMemory):
		return f.createMemoryStorage(config)
	
	case string(StorageTypeSQLite):
		return f.createSQLiteStorage(config)
		
	case string(StorageTypePostgres):
		return f.createPostgresStorage(config)
		
	case string(StorageTypeMongoDB):
		return f.createMongoDBStorage(config)
	
	default:
		return nil, fmt.Errorf("unsupported storage type: %s", config.Type)
	}
}

// createBoltDBStorage creates a BoltDB storage instance
func (f *StorageFactory) createBoltDBStorage(config FactoryConfig) (IStorage, error) {
	if config.DBPath == "" {
		return nil, fmt.Errorf("db_path is required for BoltDB storage")
	}

	storageConfig := StorageConfig{
		DBPath:   config.DBPath,
		Logger:   config.Logger,
		DeviceID: config.DeviceID,
	}

	return NewBoltStorage(storageConfig)
}

// createMemoryStorage creates an in-memory storage instance (for testing/demo)
func (f *StorageFactory) createMemoryStorage(config FactoryConfig) (IStorage, error) {
	f.logger.Warn("Memory storage is not yet implemented, falling back to BoltDB")
	// For now, create a temporary BoltDB instance
	if config.DBPath == "" {
		config.DBPath = ":memory:" // Special path for in-memory DB
	}
	return f.createBoltDBStorage(config)
}

// createSQLiteStorage creates a SQLite storage instance
func (f *StorageFactory) createSQLiteStorage(config FactoryConfig) (IStorage, error) {
	return nil, fmt.Errorf("SQLite storage is not yet implemented")
}

// createPostgresStorage creates a PostgreSQL storage instance
func (f *StorageFactory) createPostgresStorage(config FactoryConfig) (IStorage, error) {
	return nil, fmt.Errorf("PostgreSQL storage is not yet implemented")
}

// createMongoDBStorage creates a MongoDB storage instance
func (f *StorageFactory) createMongoDBStorage(config FactoryConfig) (IStorage, error) {
	return nil, fmt.Errorf("MongoDB storage is not yet implemented")
}

// GetSupportedStorageTypes returns a list of supported storage types
func (f *StorageFactory) GetSupportedStorageTypes() []StorageType {
	return []StorageType{
		StorageTypeBoltDB,
		// StorageTypeMemory,  // Uncomment when implemented
		// StorageTypeSQLite,  // Uncomment when implemented
		// StorageTypePostgres, // Uncomment when implemented
		// StorageTypeMongoDB,  // Uncomment when implemented
	}
}

// ParseStorageType parses a string into a StorageType
func ParseStorageType(s string) (StorageType, error) {
	switch strings.ToLower(s) {
	case "boltdb", "bolt":
		return StorageTypeBoltDB, nil
	case "memory", "mem":
		return StorageTypeMemory, nil
	case "sqlite", "sqlite3":
		return StorageTypeSQLite, nil
	case "postgres", "postgresql", "pg":
		return StorageTypePostgres, nil
	case "mongodb", "mongo":
		return StorageTypeMongoDB, nil
	default:
		return "", fmt.Errorf("unknown storage type: %s", s)
	}
}

// DefaultStorageConfig returns a default configuration for the specified storage type
func DefaultStorageConfig(storageType StorageType, dbPath string, logger *zap.Logger, deviceID string) FactoryConfig {
	return FactoryConfig{
		Type:     storageType,
		DBPath:   dbPath,
		Logger:   logger,
		DeviceID: deviceID,
		Options:  make(map[string]interface{}),
	}
}
