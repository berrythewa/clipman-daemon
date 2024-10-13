// File: internal/config/config_desktop.go

// +build !android,!ios

package config

import (
	"os"
	"path/filepath"
)

func init() {
	getConfigPath = desktopGetConfigPath
	getDefaultDataDir = desktopGetDefaultDataDir
}

func desktopGetConfigPath() (string, error) {
	if path := os.Getenv("CLIPMAN_CONFIG"); path != "" {
		return path, nil
	}
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "clipman", "config.json"), nil
}

func desktopGetDefaultDataDir() (string, error) {
	dataDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dataDir, ".clipman"), nil
}