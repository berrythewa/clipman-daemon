// pkg/utils/misc.go

package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
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