package storage

import (
	"time"

	"github.com/berrythewa/clipman-daemon/internal/types"
)

// Storage defines the interface for clipboard content storage operations.
// Simplified for MVP - focused on BoltDB implementation.
type Storage interface {
	// Core operations
	AddContent(content *types.ClipboardContent) error
	Query(options QueryOptions) ([]*types.ClipboardContent, error)
	// Unified query-based deletion (new approach)
	Delete(options QueryOptions) (int, error)

	// Query operations
	GetContent(id int64) (*types.ClipboardContent, error)
	GetLatestContent() (*types.ClipboardContent, error)
	GetContentSince(since time.Time) ([]*types.ClipboardContent, error)
	GetContentsByIDs(ids []int64) ([]*types.ClipboardContent, error)
	GetContentsByHashes(hashes []string) ([]*types.ClipboardContent, error)

	// Delete operations
	DeleteContent(hash string) error
	DeleteContentsByIDs(ids []int64) (int, error)
	DeleteContentsByHashes(hashes []string) (int, error)
	DeleteByTimestamp(options DeleteOptions) (int, error)
	DeleteAllContent() error

	// Statistics
	CountContent() (int, error)
	GetStats() (*StorageStats, error)

	Close() error
}
