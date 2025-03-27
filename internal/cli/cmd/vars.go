package cmd

import (
	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/pkg/utils"
	"go.uber.org/zap"
)

// Shared variables across all commands
var (
	cfg *config.Config
	zapLogger *zap.Logger

	logger *utils.Logger
	cfgFile string
	verbose bool
	quiet bool
)


// SetConfig sets the configuration for commands
func SetConfig(config *config.Config) {
	cfg = config
}

func GetConfig() *config.Config {
	return cfg.load()
}

// SetLogger sets the logger for commands
func SetLogger(log *utils.Logger) {
	logger = log
} 

func SetZapLogger(log *zap.Logger) {
	zapLogger = log
}

func GetZapLogger() *zap.Logger {
	return zapLogger
}

func GetLogger() *utils.Logger {
	return logger
}
