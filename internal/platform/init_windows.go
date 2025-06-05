//go:build windows
// +build windows

package platform

import (
	"fmt"
	"go.uber.org/zap"
	
	windowsPlatform "github.com/berrythewa/clipman-daemon/internal/platform/windows"
)

// init registers the Windows clipboard implementation
func init() {
	fmt.Println("DEBUG: Windows platform init() function called")
	defaultLogger := zap.NewNop()
	
	// Initialize clipboard
	clipboard := windowsPlatform.NewClipboard(defaultLogger)
	fmt.Printf("DEBUG: NewClipboard returned: %v (nil=%t)\n", clipboard, clipboard == nil)
	defaultClipboard = clipboard
	fmt.Printf("DEBUG: defaultClipboard set to: %v (nil=%t)\n", defaultClipboard, defaultClipboard == nil)
	
	// Initialize daemonizer
	daemonizer := windowsPlatform.NewDaemonizer()
	fmt.Printf("DEBUG: NewDaemonizer returned: %v (nil=%t)\n", daemonizer, daemonizer == nil)
	defaultDaemonizer = daemonizer
	fmt.Printf("DEBUG: defaultDaemonizer set to: %v (nil=%t)\n", defaultDaemonizer, defaultDaemonizer == nil)
} 