//go:build windows
// +build windows

package platform

import (
	"go.uber.org/zap"
)

// init registers the Windows clipboard implementation
func init() {
	defaultLogger := zap.NewNop()
	defaultClipboard = NewClipboard(defaultLogger)
	defaultDaemonizer = NewDaemonizer()
} 