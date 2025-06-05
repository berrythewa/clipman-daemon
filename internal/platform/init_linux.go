//go:build linux
// +build linux

package platform

import (
	"fmt"
	"go.uber.org/zap"
	
	linuxPlatform "github.com/berrythewa/clipman-daemon/internal/platform/linux"
)

// init registers the Linux clipboard implementation
func init() {
	fmt.Println("DEBUG: Linux platform init() function called")
	defaultLogger := zap.NewNop()
	
	// Initialize clipboard
	clipboard := linuxPlatform.NewClipboard(defaultLogger)
	fmt.Printf("DEBUG: NewClipboard returned: %v (nil=%t)\n", clipboard, clipboard == nil)
	defaultClipboard = clipboard
	fmt.Printf("DEBUG: defaultClipboard set to: %v (nil=%t)\n", defaultClipboard, defaultClipboard == nil)
	
	// Initialize daemonizer
	daemonizer := linuxPlatform.NewDaemonizer()
	fmt.Printf("DEBUG: NewDaemonizer returned: %v (nil=%t)\n", daemonizer, daemonizer == nil)
	defaultDaemonizer = daemonizer
	fmt.Printf("DEBUG: defaultDaemonizer set to: %v (nil=%t)\n", defaultDaemonizer, defaultDaemonizer == nil)
} 