// +build ios

package config

import (
	"os"
	"path/filepath"
)

func getConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, "Documents", "config.json"), nil
}

func getDefaultDataDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(homeDir, "Documents", "ClipmanData"), nil
}