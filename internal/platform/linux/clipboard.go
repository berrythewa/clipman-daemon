//go:build linux
// +build linux

package platform

import "go.uber.org/zap"

// DEPRECATED: Use EnhancedDirectClipboard instead.
// This file is kept for reference only. All logic has moved to clipboard_direct_enhanced.go.

// NewClipboard panics to indicate deprecation.
func NewClipboard(logger *zap.Logger)  {
	panic("LinuxClipboard is deprecated. Use EnhancedDirectClipboard instead.")
} 