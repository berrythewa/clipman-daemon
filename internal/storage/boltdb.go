package storage

import (
	"fmt"
	"time"

	"go.etcd.io/bbolt"
	"go.uber.org/zap"

	"github.com/berrythewa/clipman-daemon/internal/types"
)

const (
	clipboardBucket = "clipboard"
	// Optional: Use a separate bucket for ID to Hash mapping if ID lookups are critical and frequent
	idIndexBucket = "id_index"
	devicesBucket = "devices"
	deviceIDIndexBucket ="device_index"
)

// Record represents the raw data stored in BoltDB.
// This allows for internal management (like storing a hash that might be missing
// in old ClipboardContent data) without exposing these internal details.
// It maps closely to types.ClipboardContent but is the "on-disk" representation.
type Record struct {
	types.ClipboardContent // Embeds the public struct, inheriting its fields
	// Add any internal-only fields here if needed for storage
	// For example, if you stored the original key or format version.
}

// StorageConfig holds configuration for BoltStorage.
type StorageConfig struct {
	DBPath   string
	Logger   *zap.Logger
	DeviceID string
}

// Note: QueryOptions is now defined in query.go with enhanced capabilities

// BoltStorage implements the Storage interface using BoltDB.
type BoltStorage struct {
	db       *bbolt.DB
	logger   *zap.Logger
	deviceID string
}

// NewBoltStorage initializes a new BoltStorage and implements Storage.
func NewBoltStorage(config StorageConfig) (Storage, error) {
	db, err := bbolt.Open(config.DBPath, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open bolt database: %w", err)
	}

	if err := db.Update(func(tx *bbolt.Tx) error {
		// Create primary clipboard bucket
		_, err := tx.CreateBucketIfNotExists([]byte(clipboardBucket))
		if err != nil {
			return err
		}
		// Create ID index bucket (optional, for efficient ID lookups)
		_, err = tx.CreateBucketIfNotExists([]byte(idIndexBucket))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte(devicesBucket))
		if err != nil {
			return err
		}
		return err
	}); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create storage buckets: %w", err)
	}

	storage := &BoltStorage{
		db:       db,
		logger:   config.Logger,
		deviceID: config.DeviceID,
	}

	config.Logger.Debug("BoltStorage initialized", zap.String("db_path", config.DBPath), zap.String("device_id", config.DeviceID))
	return storage, nil // Return the interface type
}

// Close ends database operations.
func (s *BoltStorage) Close() error {
	s.logger.Debug("Closing BoltStorage database.")
	return s.db.Close()
}



// CountContent returns the total number of content items in the storage.
func (s *BoltStorage) CountContent() (int, error) {
	count := 0
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(clipboardBucket))
		if b == nil {
			return nil // Bucket not found means 0 items
		}
		stats := b.Stats()
		count = stats.KeyN
		return nil
	})
	if err != nil {
		s.logger.Error("Failed to count content", zap.Error(err))
		return 0, fmt.Errorf("failed to count content: %w", err)
	}
	s.logger.Debug("Content count", zap.Int("count", count))
	return count, nil
}

