// Package sync provides synchronization capabilities for clipman content
// This file provides the main entry points for the sync package
package sync

import (
	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/types"
	"go.uber.org/zap"
)

// Package version information
const (
	Version = "0.1.0"
)

// SyncMode defines the synchronization mode
type SyncMode string

// Available sync modes
const (
	ModeMQTT   SyncMode = "mqtt"   // MQTT broker-based synchronization
	ModeP2P    SyncMode = "p2p"    // Direct peer-to-peer synchronization
	ModeHybrid SyncMode = "hybrid" // Hybrid approach using both MQTT and P2P
)

// NewSyncManager creates a new SyncManager with the specified configuration
// This is the main entry point for applications using the sync package
func NewSyncManager(cfg *config.Config, logger *zap.Logger) (SyncManager, error) {
	// Initialize the sync package
	if err := Initialize(cfg, logger); err != nil {
		return nil, err
	}

	// If sync is disabled, return a no-op implementation
	if !cfg.Sync.Enabled {
		logger.Info("Sync is disabled, using no-op implementation")
		return newNoOpSyncManager(logger), nil
	}

	// Get sync manager configuration
	mode, groups, err := GetSyncManagerConfig(cfg)
	if err != nil {
		return nil, err
	}

	// Create the actual manager
	manager, err := NewManager(cfg, mode)
	if err != nil {
		logger.Error("Failed to create sync manager, falling back to no-op implementation", 
			zap.Error(err))
		return newNoOpSyncManager(logger), nil
	}

	// Log success
	logger.Info("Created sync manager", 
		zap.String("mode", string(mode)), 
		zap.Int("groups", len(groups)))

	return manager, nil
}

// Internal no-op implementations for use until proper implementations are ready

// noOpSyncManager is a placeholder implementation for SyncManager
// Used when sync is disabled or fails to initialize
type noOpSyncManager struct {
	logger *zap.Logger
}

func newNoOpSyncManager(logger *zap.Logger) *noOpSyncManager {
	return &noOpSyncManager{
		logger: logger.With(zap.String("component", "sync_manager_noop")),
	}
}

func (m *noOpSyncManager) Start() error {
	m.logger.Debug("NoOp SyncManager: Start called (no-op implementation)")
	return nil
}

func (m *noOpSyncManager) Stop() error {
	m.logger.Debug("NoOp SyncManager: Stop called (no-op implementation)")
	return nil
}

func (m *noOpSyncManager) Status() SyncStatus {
	m.logger.Debug("NoOp SyncManager: Status called (no-op implementation)")
	return SyncStatus{
		Connected: false,
		Mode:      "none",
	}
}

func (m *noOpSyncManager) SendContent(content *types.ClipboardContent, group string) error {
	m.logger.Debug("NoOp SyncManager: SendContent called (no-op implementation)",
		zap.String("group", group),
		zap.String("content_type", string(content.Type)),
		zap.Int("content_size", len(content.Data)))
	return nil
}

func (m *noOpSyncManager) SetContentHandler(handler func(*types.ClipboardContent, PeerInfo)) {
	m.logger.Debug("NoOp SyncManager: SetContentHandler called (no-op implementation)")
}

func (m *noOpSyncManager) JoinGroup(group string) error {
	m.logger.Debug("NoOp SyncManager: JoinGroup called (no-op implementation)",
		zap.String("group", group))
	return nil
}

func (m *noOpSyncManager) LeaveGroup(group string) error {
	m.logger.Debug("NoOp SyncManager: LeaveGroup called (no-op implementation)",
		zap.String("group", group))
	return nil
}

func (m *noOpSyncManager) ListGroups() ([]string, error) {
	m.logger.Debug("NoOp SyncManager: ListGroups called (no-op implementation)")
	return []string{}, nil
}

func (m *noOpSyncManager) EnableDiscovery(enabled bool) error {
	m.logger.Debug("NoOp SyncManager: EnableDiscovery called (no-op implementation)",
		zap.Bool("enabled", enabled))
	return nil
}

func (m *noOpSyncManager) GetDiscoveredPeers() []PeerInfo {
	m.logger.Debug("NoOp SyncManager: GetDiscoveredPeers called (no-op implementation)")
	return []PeerInfo{}
}

func (m *noOpSyncManager) IsConnected() bool {
	m.logger.Debug("NoOp SyncManager: IsConnected called (no-op implementation)")
	return false
} 