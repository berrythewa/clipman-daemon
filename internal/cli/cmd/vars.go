package cmd

import (
	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/pkg/utils"
)

// Shared variables across all commands
var (
	// The loaded configuration
	cfg *config.Config
	
	// Logger instance
	logger *utils.Logger
)

// SetConfig sets the configuration for commands
func SetConfig(config *config.Config) {
	cfg = config
}

// SetLogger sets the logger for commands
func SetLogger(log *utils.Logger) {
	logger = log
} 