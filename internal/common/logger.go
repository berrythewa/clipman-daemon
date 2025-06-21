package common

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// LoggerType defines the type of logger to create
type LoggerType string

const (
	// LoggerTypeCLI creates a logger for CLI operations (console + optional file)
	LoggerTypeCLI LoggerType = "cli"
	// LoggerTypeDaemon creates a logger for daemon operations (file only)
	LoggerTypeDaemon LoggerType = "daemon"
	// LoggerTypePlatform creates a logger for platform operations (file only)
	LoggerTypePlatform LoggerType = "platform"
)

// LoggerConfig holds configuration for logger creation
type LoggerConfig struct {
	Type     LoggerType
	Config   *config.Config
	LogFile  string // Optional custom log file name
	ErrorLog string // Optional custom error log file name
}

// NewLogger creates a new logger instance with human-readable timestamps
func NewLogger(cfg *config.Config) (*zap.Logger, error) {
	return NewLoggerWithConfig(LoggerConfig{
		Type:   LoggerTypeCLI,
		Config: cfg,
	})
}

// NewLoggerWithConfig creates a new logger instance with specific configuration
func NewLoggerWithConfig(logCfg LoggerConfig) (*zap.Logger, error) {
	cfg := logCfg.Config
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Parse log level
	var level zapcore.Level
	if err := level.UnmarshalText([]byte(cfg.Log.Level)); err != nil {
		level = zapcore.InfoLevel
	}

	// Create encoder config with human-readable timestamps
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
	}
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	encoderConfig.EncodeDuration = zapcore.StringDurationEncoder

	// Determine output paths based on logger type
	var outputPaths []string
	var errorOutputPaths []string

	switch logCfg.Type {
	case LoggerTypeCLI:
		// CLI logger: console + optional file
		outputPaths = append(outputPaths, "stdout")
		errorOutputPaths = append(errorOutputPaths, "stderr")

		if cfg.Log.EnableFileLogging {
			// Ensure log directory exists
			if err := os.MkdirAll(cfg.SystemPaths.LogDir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create log directory: %w", err)
			}

			// Use main clipman log files
			logFile := filepath.Join(cfg.SystemPaths.LogDir, "clipman.log")
			errorLogFile := filepath.Join(cfg.SystemPaths.LogDir, "clipman_error.log")

			outputPaths = append(outputPaths, logFile)
			errorOutputPaths = append(errorOutputPaths, errorLogFile)
		}

	case LoggerTypeDaemon:
		// Daemon logger: file only
		if cfg.Log.EnableFileLogging {
			// Ensure log directory exists
			if err := os.MkdirAll(cfg.SystemPaths.LogDir, 0755); err != nil {
				return nil, fmt.Errorf("failed to create log directory: %w", err)
			}

			// Use daemon-specific log files
			logFile := filepath.Join(cfg.SystemPaths.LogDir, "clipman_daemon.log")
			errorLogFile := filepath.Join(cfg.SystemPaths.LogDir, "clipman_daemon_error.log")

			outputPaths = append(outputPaths, logFile)
			errorOutputPaths = append(errorOutputPaths, errorLogFile)
		} else {
			// Fallback to stderr if file logging is disabled
			outputPaths = append(outputPaths, "stderr")
			errorOutputPaths = append(errorOutputPaths, "stderr")
		}

	case LoggerTypePlatform:
		// Platform logger: file only (for daemonizers)
		// Ensure log directory exists
		if err := os.MkdirAll(cfg.SystemPaths.LogDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create log directory: %w", err)
		}

		// Use platform-specific log files
		logFile := filepath.Join(cfg.SystemPaths.LogDir, "clipman_daemon.log")
		errorLogFile := filepath.Join(cfg.SystemPaths.LogDir, "clipman_daemon_error.log")

		outputPaths = append(outputPaths, logFile)
		errorOutputPaths = append(errorOutputPaths, errorLogFile)

	default:
		return nil, fmt.Errorf("unknown logger type: %s", logCfg.Type)
	}

	// Create zap config
	zapConfig := zap.Config{
		Level:             zap.NewAtomicLevelAt(level),
		Development:       false,
		DisableCaller:     false,
		DisableStacktrace: false,
		Sampling: &zap.SamplingConfig{
			Initial:    100,
			Thereafter: 100,
		},
		Encoding:         cfg.Log.Format,
		EncoderConfig:    encoderConfig,
		OutputPaths:      outputPaths,
		ErrorOutputPaths: errorOutputPaths,
	}

	// Use console encoder for human-readable output
	if cfg.Log.Format == "text" {
		zapConfig.Encoding = "console"
	}

	return zapConfig.Build()
}

// NewPlatformLogger creates a logger specifically for platform daemonizers
func NewPlatformLogger(cfg *config.Config) (*zap.Logger, error) {
	return NewLoggerWithConfig(LoggerConfig{
		Type:   LoggerTypePlatform,
		Config: cfg,
	})
}

// NewDaemonLogger creates a logger specifically for daemon operations
func NewDaemonLogger(cfg *config.Config) (*zap.Logger, error) {
	return NewLoggerWithConfig(LoggerConfig{
		Type:   LoggerTypeDaemon,
		Config: cfg,
	})
}

// NewCLILogger creates a logger specifically for CLI operations
func NewCLILogger(cfg *config.Config) (*zap.Logger, error) {
	return NewLoggerWithConfig(LoggerConfig{
		Type:   LoggerTypeCLI,
		Config: cfg,
	})
} 