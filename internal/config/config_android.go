// +build android

package config

import (
	"os"
	"path/filepath"
)

func getConfigPath() (string, error) {
	externalStorage := os.Getenv("EXTERNAL_STORAGE")
	if externalStorage == "" {
		return "", fmt.Errorf("EXTERNAL_STORAGE environment variable not set")
	}
	return filepath.Join(externalStorage, "Android", "data", "com.yourcompany.clipman", "config.json"), nil
}

func getDefaultDataDir() (string, error) {
	externalStorage := os.Getenv("EXTERNAL_STORAGE")
	if externalStorage == "" {
		return "", fmt.Errorf("EXTERNAL_STORAGE environment variable not set")
	}
	return filepath.Join(externalStorage, "Android", "data", "com.yourcompany.clipman", "data"), nil
}