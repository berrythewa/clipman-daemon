//go:build !js
// +build !js

// File: internal/config/config_desktop.go
package config

import (
	"os"
	"path/filepath"
	"runtime"
)

func init() {
	getConfigPath = getDesktopConfigPath
	getDefaultDataDir = getDesktopDataDir
}

// getDesktopConfigPath returns the path to the config file on desktop platforms
func getDesktopConfigPath() (string, error) {
	// First check environment variable
	if path := os.Getenv("CLIPMAN_CONFIG"); path != "" {
		return path, nil
	}
	
	// Get platform-specific config directory
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	
	// Use different paths for different OSes
	switch runtime.GOOS {
	case "windows":
		return filepath.Join(configDir, "Clipman", "config.json"), nil
	case "darwin":
		return filepath.Join(configDir, "com.berrythewa.clipman", "config.json"), nil
	default: // Linux and others
		return filepath.Join(configDir, "clipman", "config.json"), nil
	}
}

// getDesktopDataDir returns the path to the data directory on desktop platforms
func getDesktopDataDir() (string, error) {
	// First check environment variable
	if path := os.Getenv("CLIPMAN_DATA_DIR"); path != "" {
		return path, nil
	}
	
	// Get platform-specific home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	
	// Use different paths for different OSes
	switch runtime.GOOS {
	case "windows":
		appData, err := os.UserConfigDir()
		if err == nil {
			return filepath.Join(appData, "Clipman", "Data"), nil
		}
		return filepath.Join(homeDir, "AppData", "Local", "Clipman"), nil
	case "darwin":
		libraryDir := filepath.Join(homeDir, "Library")
		return filepath.Join(libraryDir, "Application Support", "Clipman"), nil
	default: // Linux and others
		// Check if XDG_DATA_HOME is set
		if xdgDataHome := os.Getenv("XDG_DATA_HOME"); xdgDataHome != "" {
			return filepath.Join(xdgDataHome, "clipman"), nil
		}
		return filepath.Join(homeDir, ".clipman"), nil
	}
}