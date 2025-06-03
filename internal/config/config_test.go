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

	"gopkg.in/yaml.v3"
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
		return filepath.Join(tempDir, "config.yaml"), nil
	}
	getDefaultDataDir = func() (string, error) {
		return filepath.Join(tempDir, "data"), nil
	}
	generateDeviceID = func() string {
		return "mock-device-id"
	}

	// Test loading default config when file doesn't exist
	configPath, _ := getConfigPath()
	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() failed: %v", err)
	}

	defaultCfg := DefaultConfig()
	if cfg.Log.Level != defaultCfg.Log.Level {
		t.Errorf("Expected Log.Level %s, got %s", defaultCfg.Log.Level, cfg.Log.Level)
	}
	if cfg.PollingInterval != defaultCfg.PollingInterval {
		t.Errorf("Expected PollingInterval %v, got %v", defaultCfg.PollingInterval, cfg.PollingInterval)
	}
	if cfg.SystemPaths.DataDir != filepath.Join(tempDir, "data") {
		t.Errorf("Expected DataDir %s, got %s", filepath.Join(tempDir, "data"), cfg.SystemPaths.DataDir)
	}
	if cfg.DeviceID != "mock-device-id" {
		t.Errorf("Expected DeviceID %s, got %s", "mock-device-id", cfg.DeviceID)
	}

	// Test loading existing config
	testConfig := Config{
		DeviceID:        "existing-device-id",
		DeviceName:      "test-device",
		EnableLogging:   true,
		PollingInterval: 5,
		SystemPaths: ConfigPaths{
			DataDir: filepath.Join(tempDir, "custom-data-dir"),
		},
		Log: LogConfig{
			Level: "debug",
		},
	}
	configPath, _ = getConfigPath()
	file, _ := os.Create(configPath)
	json.NewEncoder(file).Encode(testConfig)
	file.Close()

	cfg, err = Load(configPath)
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
		return filepath.Join(tempDir, "config.yaml"), nil
	}

	// Test saving config
	testConfig := &Config{
		DeviceID:        "testdevice",
		DeviceName:      "test-device",
		EnableLogging:   true,
		PollingInterval: 5,
		SystemPaths: ConfigPaths{
			DataDir: "/custom/data/dir",
		},
		Log: LogConfig{
			Level: "debug",
		},
	}

	configPath, _ := getConfigPath()
	err = testConfig.Save(configPath)
	if err != nil {
		t.Fatalf("Save() failed: %v", err)
	}

	// Verify saved config
	file, err := os.Open(configPath)
	if err != nil {
		t.Fatalf("Failed to open saved config: %v", err)
	}
	defer file.Close()

	var loadedConfig Config
	err = yaml.NewDecoder(file).Decode(&loadedConfig)
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
		return filepath.Join(tempDir, "config.yaml"), nil
	}
	getDefaultDataDir = func() (string, error) {
		return filepath.Join(tempDir, "data"), nil
	}

	// Test loading malformed config
	configPath, _ := getConfigPath()
	err = os.WriteFile(configPath, []byte("invalid yaml"), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid config: %v", err)
	}

	_, err = Load(configPath)
	if err == nil {
		t.Error("Load() should fail with invalid YAML")
	}

	// Test error in getConfigPath
	getConfigPath = func() (string, error) {
		return "", os.ErrPermission
	}

	_, err = Load("")
	if err == nil {
		t.Error("Load() should fail when getConfigPath fails")
	}

	// Test error in getDefaultDataDir
	getConfigPath = func() (string, error) {
		return filepath.Join(tempDir, "config.yaml"), nil
	}
	getDefaultDataDir = func() (string, error) {
		return "", os.ErrPermission
	}

	_, err = Load(configPath)
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
        return filepath.Join(tempDir, "config.yaml"), nil
    }
    getDefaultDataDir = func() (string, error) {
        return filepath.Join(tempDir, "data"), nil
    }

    // Test with a successful UUID generation
    generateDeviceID = func() string {
        return "successful-uuid"
    }

    configPath, _ := getConfigPath()
    cfg, err := Load(configPath)
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

    configPath, _ = getConfigPath()
    cfg, err = Load(configPath)
    if err != nil {
        t.Fatalf("Load() failed: %v", err)
    }
    if !strings.HasPrefix(cfg.DeviceID, "error-generating-uuid-") {
        t.Errorf("Expected DeviceID to start with 'error-generating-uuid-', got %s", cfg.DeviceID)
    }
}