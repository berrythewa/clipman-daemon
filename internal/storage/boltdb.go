package storage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/broker"
	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/berrythewa/clipman-daemon/pkg/compression"
	"github.com/berrythewa/clipman-daemon/pkg/utils"

	"go.etcd.io/bbolt"
)

const (
	clipboardBucket = "clipboard"
	defaultMaxSize  = 100 * 1024 * 1024 // 100MB default cache size
	keepItems       = 10                // Number of items to keep when flushing
)

// BoltStorageInterface defines the methods for BoltStorage
type BoltStorageInterface interface {
	SaveContent(content *types.ClipboardContent) error
	GetLatestContent() (*types.ClipboardContent, error)
	GetContentSince(since time.Time) ([]*types.ClipboardContent, error)
	Close() error
	GetCacheSize() int64
	FlushCache() error
}

type BoltStorage struct {
	db         *bbolt.DB
	cacheSize  int64
	maxSize    int64
	logger     *utils.Logger
	deviceID   string
	mqttClient *broker.MQTTClient
}

type StorageConfig struct {
	DBPath     string
	MaxSize    int64
	DeviceID   string
	Logger     *utils.Logger
	MQTTClient *broker.MQTTClient
}


func NewBoltStorage(config StorageConfig) (*BoltStorage, error) {
	if config.MaxSize == 0 {
		config.MaxSize = defaultMaxSize
	}

	db, err := bbolt.Open(config.DBPath, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}

	storage := &BoltStorage{
		db:       db,
		maxSize:  config.MaxSize,
		logger:   config.Logger,
		deviceID: config.DeviceID,
	}

	// Initialize buckets and calculate initial cache size
	err = db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(clipboardBucket))
		if err != nil {
			return err
		}

		// Calculate initial cache size
		var size int64
		b.ForEach(func(k, v []byte) error {
			size += int64(len(v))
			return nil
		})
		atomic.StoreInt64(&storage.cacheSize, size)

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to initialize storage: %v", err)
	}

	return storage, nil
}

func (s *BoltStorage) SaveContent(content *types.ClipboardContent) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(clipboardBucket))

		encoded, err := json.Marshal(content)
		if err != nil {
			return fmt.Errorf("failed to encode content: %v", err)
		}

		// Check cache size before adding
		newSize := atomic.AddInt64(&s.cacheSize, int64(len(encoded)))
		if newSize > s.maxSize {
			if err := s.flushOldestContent(tx); err != nil {
				s.logger.Error("Failed to flush cache", "error", err)
			}
		}

		return b.Put([]byte(content.Created.Format(time.RFC3339Nano)), encoded)
	})
}

func (s *BoltStorage) GetLatestContent() (*types.ClipboardContent, error) {
	var content *types.ClipboardContent
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(clipboardBucket))
		c := b.Cursor()
		_, v := c.Last()
		if v == nil {
			return nil
		}
		return json.Unmarshal(v, &content)
	})

	if err != nil {
		return nil, err
	}

	if content != nil && content.Compressed {
		decompressed, err := compression.DecompressContent(content)
		if err != nil {
			return nil, fmt.Errorf("failed to decompress content: %v", err)
		}
		content = decompressed
	}

	return content, nil
}

func (s *BoltStorage) GetContentSince(since time.Time) ([]*types.ClipboardContent, error) {
	var contents []*types.ClipboardContent
	err := s.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(clipboardBucket))
		c := b.Cursor()
		min := []byte(since.Format(time.RFC3339Nano))

		for k, v := c.Seek(min); k != nil; k, v = c.Next() {
			var content types.ClipboardContent
			if err := json.Unmarshal(v, &content); err != nil {
				return fmt.Errorf("failed to decode content: %v", err)
			}

			if content.Compressed {
				decompressed, err := compression.DecompressContent(&content)
				if err != nil {
					return fmt.Errorf("failed to decompress content: %v", err)
				}
				contents = append(contents, decompressed)
			} else {
				contents = append(contents, &content)
			}
		}
		return nil
	})

	return contents, err
}

func (s *BoltStorage) flushOldestContent(tx *bbolt.Tx) error {
	b := tx.Bucket([]byte(clipboardBucket))
	c := b.Cursor()

	// Collect the latest items to keep
	var keepKeys [][]byte
	for k, _ := c.Last(); k != nil && len(keepKeys) < keepItems; k, _ = c.Prev() {
		keepKeys = append(keepKeys, k)
	}

	// Collect items to be flushed for MQTT publishing
	var itemsToFlush []*types.ClipboardContent
	for k, v := c.First(); k != nil; k, v = c.Next() {
		shouldKeep := false
		for _, keep := range keepKeys {
			if bytes.Equal(k, keep) {
				shouldKeep = true
				break
			}
		}

		if !shouldKeep {
			var content types.ClipboardContent
			if err := json.Unmarshal(v, &content); err != nil {
				s.logger.Error("Failed to unmarshal content for MQTT", "error", err)
				continue
			}
			itemsToFlush = append(itemsToFlush, &content)
		}
	}

	// Publish to MQTT before deleting
	if len(itemsToFlush) > 0 && s.mqttClient != nil {
		cacheMsg := &types.CacheMessage{
			DeviceID:  s.deviceID,
			Contents:  itemsToFlush,
			Timestamp: time.Now(),
		}

		if err := s.mqttClient.PublishCache(cacheMsg); err != nil {
			s.logger.Error("Failed to publish cache to MQTT", "error", err)
			// Continue with deletion anyway
		} else {
			s.logger.Info("Published cache to MQTT",
				"items_count", len(itemsToFlush),
				"device_id", s.deviceID)
		}
	}

	// Now delete the items
	var totalFreed int64
	for _, content := range itemsToFlush {
		key := []byte(content.Created.Format(time.RFC3339Nano))
		if err := b.Delete(key); err != nil {
			return err
		}
		totalFreed += int64(len(content.Data))
	}

	atomic.AddInt64(&s.cacheSize, -totalFreed)
	s.logger.Info("Cache flushed",
		"freed_bytes", totalFreed,
		"remaining_items", len(keepKeys),
		"published_items", len(itemsToFlush))

	return nil
}

func (s *BoltStorage) PublishCacheHistory(since time.Time) error {
	if s.mqttClient == nil {
		return fmt.Errorf("MQTT client not configured")
	}

	contents, err := s.GetContentSince(since)
	if err != nil {
		return fmt.Errorf("failed to get contents: %v", err)
	}

	if len(contents) == 0 {
		return nil
	}

	cacheMsg := &types.CacheMessage{
		DeviceID:  s.deviceID,
		Contents:  contents,
		Timestamp: time.Now(),
	}

	return s.mqttClient.PublishCache(cacheMsg)
}

func (s *BoltStorage) GetCacheSize() int64 {
	return atomic.LoadInt64(&s.cacheSize)
}

func (s *BoltStorage) FlushCache() error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		return s.flushOldestContent(tx)
	})
}

func (s *BoltStorage) Close() error {
	// Attempt to flush before closing
	if err := s.FlushCache(); err != nil {
		s.logger.Error("Failed to flush cache on close", "error", err)
	}
	return s.db.Close()
}
