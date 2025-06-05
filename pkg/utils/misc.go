// pkg/utils/misc.go

package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/types"
)

func Min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func Max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// ExpandPath expands ~ to home directory and resolves relative paths
func ExpandPath(path string) (string, error) {
	if path == "" {
		return "", nil
	}

	// Expand ~ to home directory
	if strings.HasPrefix(path, "~/") {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		path = filepath.Join(homeDir, path[2:])
	} else if path == "~" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		path = homeDir
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	return absPath, nil
}

// MustExpandPath expands a path or panics if it fails (for use with known-good paths)
func MustExpandPath(path string) string {
	expanded, err := ExpandPath(path)
	if err != nil {
		panic(fmt.Sprintf("failed to expand path %s: %v", path, err))
	}
	return expanded
}

// EnsureDir creates a directory if it doesn't exist
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// HashContent creates a SHA-256 hash string of content for change detection
func HashContent(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// HashContentBis creates a simple checksum-based hash for large content
func HashContentBis(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	var hash uint32
	for i, b := range data {
		hash = (hash << 5) + hash + uint32(b)
		if i > 4096 {
			break
		}
	}
	return fmt.Sprintf("%x", hash)
}

// NewClipboardContent creates a new ClipboardContent with hash and occurrences
func NewClipboardContent(contentType types.ContentType, data []byte) *types.ClipboardContent {
	now := time.Now()
	return &types.ClipboardContent{
		Type:        contentType,
		Data:        data,
		Hash:        HashContent(data),
		Created:     now,
		Occurrences: []time.Time{now},
	}
}