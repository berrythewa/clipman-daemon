// File: internal/config/config_test.go

package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

)

func TestLoad(t *testing.T) {
	// Setup
	tempDir, err := os.MkdirTemp("", "clipman-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Save original functions and restore them after the test
	origGetConfigPath := getConfigPath
	origGetDefaultDataDir := getDefaultDataDir
	origGenerateDeviceID := generateDeviceID
	defer func() {
		getConfigPath = origGetConfigPath
		getDefaultDataDir = origGetDefaultDataDir
		generateDeviceID = origGenerateDeviceID
	}()

	// Set up mocks
	getConfigPath = func() (string, error) {
		return filepath.Join(tempDir, "config.json"), nil
	}
	getDefaultDataDir = func() (string, error) {
		return filepath.Join(tempDir, "data"), nil
	}
	generateDeviceID = func() string {
		return "mock-device-id"
	}

	// Test loading default config when file doesn't exist
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if cfg.LogLevel != DefaultConfig.LogLevel {
		t.Errorf("Expected LogLevel %s, got %s", DefaultConfig.LogLevel, cfg.LogLevel)
	}
	if cfg.PollingInterval != DefaultConfig.PollingInterval {
		t.Errorf("Expected PollingInterval %v, got %v", DefaultConfig.PollingInterval, cfg.PollingInterval)
	}
	if cfg.DataDir != filepath.Join(tempDir, "data") {
		t.Errorf("Expected DataDir %s, got %s", filepath.Join(tempDir, "data"), cfg.DataDir)
	}
	if cfg.DeviceID != "mock-device-id" {
		t.Errorf("Expected DeviceID %s, got %s", "mock-device-id", cfg.DeviceID)
	}

	// Test loading existing config
	testConfig := Config{
		LogLevel:        "debug",
		BrokerURL:       "mqtt://test.mosquitto.org",
		BrokerUsername:  "testuser",
		BrokerPassword:  "testpass",
		DeviceID:        "existing-device-id",
		PollingInterval: 5 * time.Second,
		DataDir:         filepath.Join(tempDir, "custom-data-dir"), // Use a subdirectory of tempDir
	}
	configPath, _ := getConfigPath()
	file, _ := os.Create(configPath)
	json.NewEncoder(file).Encode(testConfig)
	file.Close()

	cfg, err = Load()
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}
	if !reflect.DeepEqual(cfg, &testConfig) {
		t.Errorf("Loaded config doesn't match saved config. Got %+v, want %+v", cfg, testConfig)
	}
}
func TestSave(t *testing.T) {
	// Setup
	tempDir, err := os.MkdirTemp("", "clipman-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Save original function and restore it after the test
	origGetConfigPath := getConfigPath
	defer func() { getConfigPath = origGetConfigPath }()

	// Set up mock
	getConfigPath = func() (string, error) {
		return filepath.Join(tempDir, "config.json"), nil
	}

	// Test saving config
	testConfig := &Config{
		LogLevel:        "debug",
		BrokerURL:       "mqtt://test.mosquitto.org",
		BrokerUsername:  "testuser",
		BrokerPassword:  "testpass",
		DeviceID:        "testdevice",
		PollingInterval: 5 * time.Second,
		DataDir:         "/custom/data/dir",
	}

	err = testConfig.Save()
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Verify saved config
	configPath, _ := getConfigPath()
	file, err := os.Open(configPath)
	if err != nil {
		t.Fatalf("Failed to open saved config: %v", err)
	}
	defer file.Close()

	var loadedConfig Config
	err = json.NewDecoder(file).Decode(&loadedConfig)
	if err != nil {
		t.Fatalf("Failed to decode saved config: %v", err)
	}

	if !reflect.DeepEqual(testConfig, &loadedConfig) {
		t.Errorf("Saved config doesn't match original. Got %+v, want %+v", loadedConfig, testConfig)
	}
}

func TestLoadConfigErrorHandling(t *testing.T) {
	// Setup
	tempDir, err := os.MkdirTemp("", "clipman-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Save original functions and restore them after the test
	origGetConfigPath := getConfigPath
	origGetDefaultDataDir := getDefaultDataDir
	defer func() {
		getConfigPath = origGetConfigPath
		getDefaultDataDir = origGetDefaultDataDir
	}()

	// Set up initial mocks
	getConfigPath = func() (string, error) {
		return filepath.Join(tempDir, "config.json"), nil
	}
	getDefaultDataDir = func() (string, error) {
		return filepath.Join(tempDir, "data"), nil
	}

	// Test loading malformed config
	configPath, _ := getConfigPath()
	err = os.WriteFile(configPath, []byte("invalid json"), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid config: %v", err)
	}

	_, err = Load()
	if err == nil {
		t.Error("Load() should fail with invalid JSON")
	}

	// Test error in getConfigPath
	getConfigPath = func() (string, error) {
		return "", os.ErrPermission
	}

	_, err = Load()
	if err == nil {
		t.Error("Load() should fail when getConfigPath fails")
	}

	// Test error in getDefaultDataDir
	getConfigPath = func() (string, error) {
		return filepath.Join(tempDir, "config.json"), nil
	}
	getDefaultDataDir = func() (string, error) {
		return "", os.ErrPermission
	}

	_, err = Load()
	if err == nil {
		t.Error("Load() should fail when getDefaultDataDir fails")
	}
}

func TestDeviceIDGeneration(t *testing.T) {
    // Save original functions and restore them after the test
    origGetConfigPath := getConfigPath
    origGetDefaultDataDir := getDefaultDataDir
    origGenerateDeviceID := generateDeviceID
    defer func() {
        getConfigPath = origGetConfigPath
        getDefaultDataDir = origGetDefaultDataDir
        generateDeviceID = origGenerateDeviceID
    }()

    // Set up mocks
    tempDir, _ := os.MkdirTemp("", "clipman-test")
    defer os.RemoveAll(tempDir)

    getConfigPath = func() (string, error) {
        return filepath.Join(tempDir, "config.json"), nil
    }
    getDefaultDataDir = func() (string, error) {
        return filepath.Join(tempDir, "data"), nil
    }

    // Test with a successful UUID generation
    generateDeviceID = func() string {
        return "successful-uuid"
    }

    cfg, err := Load()
    if err != nil {
        t.Fatalf("Load() failed: %v", err)
    }
    if cfg.DeviceID != "successful-uuid" {
        t.Errorf("Expected DeviceID %s, got %s", "successful-uuid", cfg.DeviceID)
    }

    // Test with a failed UUID generation
    generateDeviceID = func() string {
        return fmt.Sprintf("error-generating-uuid-%d", time.Now().UnixNano())
    }

    cfg, err = Load()
    if err != nil {
        t.Fatalf("Load() failed: %v", err)
    }
    if !strings.HasPrefix(cfg.DeviceID, "error-generating-uuid-") {
        t.Errorf("Expected DeviceID to start with 'error-generating-uuid-', got %s", cfg.DeviceID)
    }
}