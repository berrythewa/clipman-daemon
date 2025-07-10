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
	
	// Register the enhanced direct clipboard factory with config support
	RegisterClipboardFactoryWithConfig(func(logger *zap.Logger, config interface{}) Clipboard {
		fmt.Printf("DEBUG: Creating enhanced direct clipboard with config: %v\n", config != nil)
		
		// Extract HTML processing config from the provided config
		htmlConfig := linuxPlatform.HTMLProcessingConfig{
			ExtractText:         false,
			PreferExtractedText: true,
			KeepBoth:            false,
		}
		
		// Try to extract HTML config from the provided config
		if configMap, ok := config.(map[string]interface{}); ok {
			if htmlConfigMap, exists := configMap["html_processing"]; exists {
				if htmlMap, ok := htmlConfigMap.(map[string]interface{}); ok {
					if extractText, exists := htmlMap["extract_text"]; exists {
						if extract, ok := extractText.(bool); ok {
							htmlConfig.ExtractText = extract
						}
					}
					if preferText, exists := htmlMap["prefer_extracted_text"]; exists {
						if prefer, ok := preferText.(bool); ok {
							htmlConfig.PreferExtractedText = prefer
						}
					}
					if keepBoth, exists := htmlMap["keep_both"]; exists {
						if keep, ok := keepBoth.(bool); ok {
							htmlConfig.KeepBoth = keep
						}
					}
				}
			}
		}
		
		clipboard, err := linuxPlatform.NewEnhancedDirectClipboardWithConfig(logger, htmlConfig)
		if err != nil {
			panic(fmt.Sprintf("Failed to create enhanced direct clipboard with config: %v", err))
		}
		fmt.Printf("DEBUG: Enhanced direct clipboard with config created successfully\n")
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