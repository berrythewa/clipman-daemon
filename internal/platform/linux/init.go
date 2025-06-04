//go:build linux
// +build linux

package platform

import (
	"go.uber.org/zap"
)

// init registers the Linux clipboard implementation
func init() {
	defaultLogger := zap.NewNop()
	defaultClipboard = NewClipboard(defaultLogger)
} 