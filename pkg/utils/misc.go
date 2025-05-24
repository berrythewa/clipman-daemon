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
