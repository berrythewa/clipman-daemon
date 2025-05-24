package utils

import (
	"crypto/sha256"
	"encoding/hex"
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

func hashContent(data []byte) string {
    h := sha256.Sum256(data)
    return hex.EncodeToString(h[:])
}

// hashContent creates a simple hash string of content for change detection
func hashContentBis(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	
	// Simple checksum-based hash that doesn't need to access clipboard again
	var hash uint32
	for i, b := range data {
		hash = (hash << 5) + hash + uint32(b)
		// Only use first 4KB for hashing large content
		if i > 4096 {
			break
		}
	}
	return fmt.Sprintf("%x", hash)
}