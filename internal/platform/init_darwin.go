//go:build darwin
// +build darwin

package platform

import (
	"fmt"
	"go.uber.org/zap"
	
	darwinPlatform "github.com/berrythewa/clipman-daemon/internal/platform/darwin"
)

// init registers the Darwin (macOS) clipboard implementation
func init() {
	fmt.Println("DEBUG: Darwin (macOS) platform init() function called")
	defaultLogger := zap.NewNop()
	
	// Initialize clipboard
	clipboard := darwinPlatform.NewClipboard(defaultLogger)
	fmt.Printf("DEBUG: NewClipboard returned: %v (nil=%t)\n", clipboard, clipboard == nil)
	defaultClipboard = clipboard
	fmt.Printf("DEBUG: defaultClipboard set to: %v (nil=%t)\n", defaultClipboard, defaultClipboard == nil)
	
	// Initialize daemonizer
	daemonizer := darwinPlatform.NewDaemonizer()
	fmt.Printf("DEBUG: NewDaemonizer returned: %v (nil=%t)\n", daemonizer, daemonizer == nil)
	defaultDaemonizer = daemonizer
	fmt.Printf("DEBUG: defaultDaemonizer set to: %v (nil=%t)\n", defaultDaemonizer, defaultDaemonizer == nil)
} 