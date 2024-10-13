// File: internal/storage/boltdb_test.go

package storage

import (
    "os"
    "testing"
    "time"

    "github.com/berrythewa/clipman-daemon/internal/types"
    "github.com/berrythewa/clipman-daemon/pkg/compression"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
    "go.etcd.io/bbolt"
)

func TestBoltStorage(t *testing.T) {
    // Create a temporary file for the test database
    tmpfile, err := os.CreateTemp("", "test_bolt_*.db")
    require.NoError(t, err)
    defer os.Remove(tmpfile.Name())

    // Create a new BoltStorage instance
    storage, err := NewBoltStorage(tmpfile.Name())
    require.NoError(t, err)
    defer storage.Close()

    // Test SaveContent and GetLatestContent
    t.Run("SaveAndGetLatestContent", func(t *testing.T) {
        content := &types.ClipboardContent{
            Type:    types.TypeText,
            Data:    "Test content",
            Created: time.Now(),
        }

        err = storage.SaveContent(content)
        assert.NoError(t, err)

        retrievedContent, err := storage.GetLatestContent()
        assert.NoError(t, err)
        assert.Equal(t, content.Type, retrievedContent.Type)
        assert.Equal(t, content.Data, retrievedContent.Data)
        assert.WithinDuration(t, content.Created, retrievedContent.Created, time.Second)
    })

    // Test GetContentSince
    t.Run("GetContentSince", func(t *testing.T) {
        start := time.Now()
        content1 := &types.ClipboardContent{
            Type:    types.TypeText,
            Data:    "Content 1",
            Created: start.Add(time.Second),
        }
        content2 := &types.ClipboardContent{
            Type:    types.TypeText,
            Data:    "Content 2",
            Created: start.Add(2 * time.Second),
        }

        err = storage.SaveContent(content1)
        assert.NoError(t, err)
        err = storage.SaveContent(content2)
        assert.NoError(t, err)

        contents, err := storage.GetContentSince(start)
        assert.NoError(t, err)
        assert.Len(t, contents, 2)
        assert.Equal(t, content1.Data, contents[0].Data)
        assert.Equal(t, content2.Data, contents[1].Data)
    })

    // Test compressed content
    t.Run("CompressedContent", func(t *testing.T) {
        originalContent := &types.ClipboardContent{
            Type:    types.TypeText,
            Data:    "This is some test content that will be compressed",
            Created: time.Now(),
        }

        compressedData, err := compression.compressContent([]byte(originalContent.Data))
        require.NoError(t, err)

        compressedContent := &types.ClipboardContent{
            Type:       originalContent.Type,
            Data:       string(compressedData),
            Created:    originalContent.Created,
            Compressed: true,
        }

        err = storage.SaveContent(compressedContent)
        assert.NoError(t, err)

        retrievedContent, err := storage.GetLatestContent()
        assert.NoError(t, err)
        assert.Equal(t, originalContent.Type, retrievedContent.Type)
        assert.Equal(t, originalContent.Data, retrievedContent.Data)
        assert.WithinDuration(t, originalContent.Created, retrievedContent.Created, time.Second)
        assert.False(t, retrievedContent.Compressed)
    })
    // Test error cases
    t.Run("ErrorCases", func(t *testing.T) {
        // Test getting content from empty database
        storage.db.Update(func(tx *bbolt.Tx) error {
            return tx.DeleteBucket([]byte(clipboardBucket))
        })

        _, err := storage.GetLatestContent()
        assert.Error(t, err)

        // Test saving invalid content
        invalidContent := &types.ClipboardContent{
            Type:    types.TypeText,
            Data:    string([]byte{0xFF, 0xFE}), // Invalid UTF-8
            Created: time.Now(),
        }
        err = storage.SaveContent(invalidContent)
        assert.Error(t, err)
    })
}