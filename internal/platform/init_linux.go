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
	
	// Register the enhanced direct clipboard factory for creating clipboards with proper loggers
	RegisterClipboardFactory(func(logger *zap.Logger) Clipboard {
		fmt.Printf("DEBUG: Creating enhanced direct clipboard with logger: %v\n", logger != nil)
		clipboard, err := linuxPlatform.NewEnhancedDirectClipboard(logger)
		if err != nil {
			panic(fmt.Sprintf("Failed to create enhanced direct clipboard: %v", err))
		}
		fmt.Printf("DEBUG: Enhanced direct clipboard created successfully\n")
		return clipboard
	})
	
	// Initialize default clipboard with enhanced direct implementation
	var clipboard Clipboard
	clipboard, err := linuxPlatform.NewEnhancedDirectClipboard(defaultLogger)
	if err != nil {
		panic(fmt.Sprintf("Failed to create enhanced direct clipboard for default: %v", err))
	}
	fmt.Printf("DEBUG: NewEnhancedDirectClipboard returned: %v (nil=%t)\n", clipboard, clipboard == nil)
	defaultClipboard = clipboard
	fmt.Printf("DEBUG: defaultClipboard set to: %v (nil=%t)\n", defaultClipboard, defaultClipboard == nil)
	
	// Initialize daemonizer
	daemonizer := linuxPlatform.NewDaemonizer()
	fmt.Printf("DEBUG: NewDaemonizer returned: %v (nil=%t)\n", daemonizer, daemonizer == nil)
	defaultDaemonizer = daemonizer
	fmt.Printf("DEBUG: defaultDaemonizer set to: %v (nil=%t)\n", defaultDaemonizer, defaultDaemonizer == nil)
} 