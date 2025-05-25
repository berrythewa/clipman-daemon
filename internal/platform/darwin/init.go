//go:build darwin
// +build darwin

package platform

import (
	"go.uber.org/zap"
)

// init registers the Darwin (macOS) clipboard implementation
func init() {
	defaultLogger := zap.NewNop()
	defaultClipboard = NewClipboard(defaultLogger)
	defaultDaemonizer = NewDaemonizer()
} 