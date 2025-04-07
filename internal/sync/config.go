// Package sync provides synchronization capabilities for clipman content
// This file contains sync-specific constants, types, and helper functions
package sync

import (
	"fmt"

	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/types"
	"go.uber.org/zap"
)

// SyncStatus represents the current status of synchronization
type SyncStatus string

// SyncStatus constants
const (
	SyncStatusDisconnected SyncStatus = "disconnected"
	SyncStatusConnecting   SyncStatus = "connecting"
	SyncStatusConnected    SyncStatus = "connected"
	SyncStatusError        SyncStatus = "error"
)

// MessageHandler processes sync messages
type MessageHandler func(Message)

// ContentHandlerFunc defines a function type for handling received content
type ContentHandlerFunc func(*types.ClipboardContent, PeerInfo)

// Initialize sets up the sync package with configuration from the central config
func Initialize(cfg *config.Config, logger *zap.Logger) error {
	// Validate configuration
	if !cfg.Sync.Enabled {
		logger.Info("Sync functionality is disabled")
		return nil
	}

	// Log sync configuration
	logger.Info("Initializing sync package",
		zap.String("mode", cfg.Sync.Mode),
		zap.String("protocol", cfg.Sync.Protocol),
		zap.String("default_group", cfg.Sync.DefaultGroup),
		zap.Bool("discoverable", cfg.Sync.Discoverable))

	// Protocol validation
	protocol := cfg.Sync.Protocol
	if protocol == "" {
		// Default to MQTT if not specified
		protocol = string(ModeMQTT)
	}

	// Protocol-specific validation
	switch SyncMode(protocol) {
	case ModeMQTT:
		if cfg.Sync.URL == "" {
			return fmt.Errorf("MQTT broker URL must be specified")
		}
	case ModeP2P:
		// No specific requirements for P2P yet
	case ModeHybrid:
		if cfg.Sync.URL == "" {
			return fmt.Errorf("MQTT broker URL must be specified for hybrid mode")
		}
	default:
		return fmt.Errorf("unknown protocol: %s", protocol)
	}

	return nil
}

// GetSyncManagerConfig extracts relevant configuration for creating a SyncManager
func GetSyncManagerConfig(cfg *config.Config) (SyncMode, []string, error) {
	// Get sync mode from config
	var mode SyncMode
	switch cfg.Sync.Mode {
	case "centralized", "":
		mode = ModeMQTT
	case "p2p":
		mode = ModeP2P
	case "hybrid":
		mode = ModeHybrid
	default:
		return "", nil, fmt.Errorf("unknown sync mode: %s", cfg.Sync.Mode)
	}

	// Get groups from config
	groups := []string{}
	if cfg.Sync.DefaultGroup != "" {
		groups = append(groups, cfg.Sync.DefaultGroup)
	}

	return mode, groups, nil
} 