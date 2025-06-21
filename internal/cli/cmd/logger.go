package cmd

import (
	"fmt"

	"github.com/berrythewa/clipman-daemon/internal/common"
	"github.com/berrythewa/clipman-daemon/internal/config"
	"go.uber.org/zap"
)

// SetupLogger creates a zap logger with configurable file output and human-readable timestamps
func SetupLogger(cfg *config.Config) (*zap.Logger, error) {
	return common.NewCLILogger(cfg)
}

// GetLogger returns the configured logger, creating it if necessary
func GetLogger() (*zap.Logger, error) {
	if zapLogger != nil {
		return zapLogger, nil
	}

	if cfg == nil {
		return nil, fmt.Errorf("configuration not loaded")
	}

	logger, err := SetupLogger(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to setup logger: %w", err)
	}

	zapLogger = logger
	return logger, nil
} 