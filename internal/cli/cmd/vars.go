package cmd

import (
	"github.com/berrythewa/clipman-daemon/internal/config"
	"go.uber.org/zap"
)

// Shared variables across all commands
var (
	cfg *config.Config
	zapLogger *zap.Logger

	cfgFile string
	verbose bool
	quiet bool
)


// SetConfig sets the configuration for commands
func SetConfig(config *config.Config) {
	cfg = config
}

func GetConfig() *config.Config {
	return cfg
}

// SetZapLogger sets the logger for commands
func SetZapLogger(log *zap.Logger) {
	zapLogger = log
}

func GetZapLogger() *zap.Logger {
	return zapLogger
}
