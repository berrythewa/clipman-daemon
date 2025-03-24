// File: pkg/utils/logger_test.go

package utils

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNewLogger(t *testing.T) {
	tests := []struct {
		level    string
		expected LogLevel
	}{
		{"debug", LogLevelDebug},
		{"info", LogLevelInfo},
		{"warn", LogLevelWarn},
		{"error", LogLevelError},
		{"fatal", LogLevelFatal},
		{"invalid", LogLevelInfo}, // default to Info for invalid levels
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("Level_%s", test.level), func(t *testing.T) {
			logger := NewLogger(LoggerOptions{Level: test.level})
			if logger.level != test.expected {
				t.Errorf("NewLogger(%s) level = %v, want %v", test.level, logger.level, test.expected)
			}
		})
	}
}

func TestLogLevels(t *testing.T) {
	buf := new(bytes.Buffer)
	logger := NewLogger(LoggerOptions{Level: "debug", Output: buf})

	tests := []struct {
		logFunc func(string, ...interface{})
		level   string
	}{
		{logger.Debug, "DEBUG"},
		{logger.Info, "INFO"},
		{logger.Warn, "WARN"},
		{logger.Error, "ERROR"},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("Level_%s", test.level), func(t *testing.T) {
			buf.Reset()
			test.logFunc("test message", "key", "value")
			output := buf.String()
			if !strings.Contains(output, test.level) {
				t.Errorf("Log output doesn't contain expected level %s. Output: %s", test.level, output)
			}
			if !strings.Contains(output, "test message") {
				t.Errorf("Log output doesn't contain expected message. Output: %s", output)
			}
			if !strings.Contains(output, "key=value") {
				t.Errorf("Log output doesn't contain expected key-value pair. Output: %s", output)
			}
		})
	}
}

func TestLogLevelFiltering(t *testing.T) {
	buf := new(bytes.Buffer)
	logger := NewLogger(LoggerOptions{Level: "warn", Output: buf})

	logger.Debug("debug message")
	logger.Info("info message")
	logger.Warn("warn message")
	logger.Error("error message")

	output := buf.String()
	if strings.Contains(output, "debug message") || strings.Contains(output, "info message") {
		t.Errorf("Logger didn't filter out debug and info messages. Output: %s", output)
	}
	if !strings.Contains(output, "warn message") || !strings.Contains(output, "error message") {
		t.Errorf("Logger filtered out warn or error messages. Output: %s", output)
	}
}

func TestFatalLog(t *testing.T) {
	t.Log("Starting TestFatalLog")
	buf := new(bytes.Buffer)
	logger := NewLogger(LoggerOptions{Level: "info", Output: buf})

	// Redefine os.Exit for this test
	oldOsExit := osExit
	defer func() { 
		osExit = oldOsExit 
		t.Log("Restored original osExit")
	}()

	var exitCalled bool
	var exitCode int
	osExit = func(code int) {
		t.Log("osExit called with code:", code)
		exitCalled = true
		exitCode = code
		// Don't actually exit
	}

	t.Log("Calling logger.Fatal")
	logger.Fatal("fatal error")
	t.Log("logger.Fatal call completed")

	if !exitCalled {
		t.Error("os.Exit was not called by Fatal")
	} else {
		t.Log("os.Exit was called")
		if exitCode != 1 {
			t.Errorf("os.Exit called with code %d, expected 1", exitCode)
		} else {
			t.Log("os.Exit called with correct code")
		}
	}

	output := buf.String()
	t.Logf("Fatal log output: %q", output)  // Use %q to show any hidden characters

	if !strings.Contains(output, "FATAL") {
		t.Error("Fatal log doesn't contain 'FATAL' level")
	} else {
		t.Log("Fatal log contains 'FATAL' level")
	}
	if !strings.Contains(output, "fatal error") {
		t.Error("Fatal log doesn't contain the expected error message")
	} else {
		t.Log("Fatal log contains the expected error message")
	}

	t.Log("TestFatalLog completed")
}

func TestFileLogging(t *testing.T) {
	// Create a temporary directory for log files
	tempDir, err := os.MkdirTemp("", "logger_test")
	if err != nil {
		t.Fatalf("Failed to create temp directory: %v", err)
	}
	defer os.RemoveAll(tempDir)
	
	// Create a logger with file logging enabled
	logger := NewLogger(LoggerOptions{
		Level:    "debug",
		LogDir:   tempDir,
		MaxSize:  1024, // Small size for testing rotation
		MaxFiles: 3,
	})
	
	// Write some log messages
	for i := 0; i < 10; i++ {
		logger.Info(fmt.Sprintf("Test log message %d", i), "count", i)
	}
	
	// Close the logger
	if err := logger.Close(); err != nil {
		t.Errorf("Failed to close logger: %v", err)
	}
	
	// Check if log file was created
	files, err := os.ReadDir(tempDir)
	if err != nil {
		t.Fatalf("Failed to read temp directory: %v", err)
	}
	
	if len(files) == 0 {
		t.Fatal("No log files were created")
	}
	
	// Verify at least one file has log content
	logFile := filepath.Join(tempDir, files[0].Name())
	content, err := os.ReadFile(logFile)
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	
	if !strings.Contains(string(content), "Test log message") {
		t.Errorf("Log file doesn't contain expected message. Content: %s", string(content))
	}
}

func TestMain(m *testing.M) {
	// This function will be called instead of the regular testing main function
	// It allows us to add setup and teardown code, and to catch panics
	defer func() {
		if r := recover(); r != nil {
			fmt.Printf("Tests panicked: %v", r)
			os.Exit(1)
		}
	}()

	// Run the tests
	exitCode := m.Run()

	// Exit with the same code as the tests
	os.Exit(exitCode)
}