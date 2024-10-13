// File: pkg/utils/logger_test.go

package utils

import (
	"bytes"
	"fmt"
	"os"
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
			logger := NewLogger(test.level, nil)
			if logger.level != test.expected {
				t.Errorf("NewLogger(%s) level = %v, want %v", test.level, logger.level, test.expected)
			}
		})
	}
}

func TestLogLevels(t *testing.T) {
	buf := new(bytes.Buffer)
	logger := NewLogger("debug", buf)

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
	logger := NewLogger("warn", buf)

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
	logger := NewLogger("info", buf)

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