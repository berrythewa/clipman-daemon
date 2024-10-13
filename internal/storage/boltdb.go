// File: internal/storage/boltdb.go

package storage

import (
    "encoding/json"
    "fmt"
    "time"

    "github.com/berrythewa/clipman-daemon/internal/types"
    "go.etcd.io/bbolt"
    "github.com/berrythewa/clipman-daemon/pkg/compression"
)

// BoltStorageInterface defines the methods for BoltStorage.
type BoltStorageInterface interface {
    SaveContent(content *types.ClipboardContent) error
    GetLatestContent() (*types.ClipboardContent, error)
    GetContentSince(since time.Time) ([]*types.ClipboardContent, error)
    Close() error
}


const (
    clipboardBucket = "clipboard"
)

type BoltStorage struct {
    db *bbolt.DB
}

func NewBoltStorage(dbPath string) (*BoltStorage, error) {
    db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{Timeout: 1 * time.Second})
    if err != nil {
        return nil, fmt.Errorf("failed to open database: %v", err)
    }
    err = db.Update(func(tx *bbolt.Tx) error {
        _, err := tx.CreateBucketIfNotExists([]byte(clipboardBucket))
        return err
    })
    if err != nil {
        return nil, fmt.Errorf("failed to create bucket: %v", err)
    }
    return &BoltStorage{db: db}, nil
}

func (s *BoltStorage) Close() error {
    return s.db.Close()
}

func (s *BoltStorage) SaveContent(content *types.ClipboardContent) error {
    return s.db.Update(func(tx *bbolt.Tx) error {
        b := tx.Bucket([]byte(clipboardBucket))
        encoded, err := json.Marshal(content)
        if err != nil {
            return fmt.Errorf("failed to encode content: %v", err)
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
            return nil // No content found
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