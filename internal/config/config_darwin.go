//go:build darwin

package config

import (
	"os"
	"path/filepath"
)

func init() {
	getConfigPath = darwinGetConfigPath
	getDefaultDataDir = darwinGetDefaultDataDir
}

func darwinGetConfigPath() (string, error) {
	configDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "Library", "Application Support", "clipman", "config.json"), nil
}

func darwinGetDefaultDataDir() (string, error) {
	configDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "Library", "Application Support", "clipman", "data"), nil
}
