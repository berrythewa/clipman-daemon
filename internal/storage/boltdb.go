package storage

import (
	"fmt"
	"time"

	"go.etcd.io/bbolt"
	"go.uber.org/zap"
)

const (
	clipboardBucket = "clipboard"
)

// BoltStorage handles clipboard storage.
type BoltStorage struct {
	db       *bbolt.DB
	logger   *zap.Logger
	deviceID string
}

// NewBoltStorage initializes a new BoltStorage.
func NewBoltStorage(config StorageConfig) (*BoltStorage, error) {
	// Open the database
	db, err := bbolt.Open(config.DBPath, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open bolt database: %w", err)
	}

	if err := createBucket(db); err != nil {
		db.Close()
		return nil, err
	}

	storage := &BoltStorage{
		db:       db,
		logger:   config.Logger,
		deviceID: config.DeviceID,
	}

	config.Logger.Debug("BoltStorage initialized")
	return storage, nil
}

func createBucket(db *bbolt.DB) error {
	return db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(clipboardBucket))
		return err
	})
}

// Close ends database operations.
func (s *BoltStorage) Close() error {
	return s.db.Close()
}
