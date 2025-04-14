// Package sync provides clipboard synchronization using libp2p
package sync

import (
	"context"
	"fmt"
	"sync"

	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/types"
	"go.uber.org/zap"
)

// Manager implements the types.SyncManager interface
// It orchestrates the libp2p node, protocols, and discovery
type Manager struct {
	// Core components
	node          *Node
	ctx           context.Context
	cancel        context.CancelFunc
	config        *SyncConfig
	logger        *zap.Logger
	
	// Content handling
	contentHandler types.ContentCallback
	handlerMutex   sync.RWMutex
	
	// State
	started       bool
	mutex         sync.RWMutex
}

// New creates a new sync manager
func New(ctx context.Context, cfg *config.Config, logger *zap.Logger) (*Manager, error) {
	// Create cancellable context
	mgrCtx, cancel := context.WithCancel(ctx)
	
	// Initialize logger
	syncLogger := logger
	if syncLogger == nil {
		syncLogger = zap.NewNop()
	}
	syncLogger = syncLogger.With(zap.String("component", "sync-manager"))
	
	// Create the node
	node, err := NewNode(mgrCtx, cfg, syncLogger)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create libp2p node: %w", err)
	}
	
	// Create the manager
	manager := &Manager{
		node:          node,
		ctx:           mgrCtx,
		cancel:        cancel,
		config:        node.GetConfig(),
		logger:        syncLogger,
		started:       false,
	}
	
	return manager, nil
}

// Start starts the sync manager
func (m *Manager) Start() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	if m.started {
		return nil
	}
	
	// Start the node
	if err := m.node.Start(); err != nil {
		return fmt.Errorf("failed to start node: %w", err)
	}
	
	// TODO: Initialize protocol handlers
	
	m.started = true
	m.logger.Info("Sync manager started")
	return nil
}

// Stop stops the sync manager
func (m *Manager) Stop() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	if !m.started {
		return nil
	}
	
	// Stop the node
	if err := m.node.Stop(); err != nil {
		return fmt.Errorf("failed to stop node: %w", err)
	}
	
	// Cancel the context
	m.cancel()
	
	m.started = false
	m.logger.Info("Sync manager stopped")
	return nil
}

// SendContent sends content to a group
func (m *Manager) SendContent(content *types.ClipboardContent, group string) error {
	if !m.started {
		return fmt.Errorf("sync manager not started")
	}
	
	// TODO: Implement content sending via protocols
	m.logger.Info("Sending content to group", 
		zap.String("group", group),
		zap.String("content_type", string(content.Type)))
	
	return fmt.Errorf("not implemented")
}

// SetContentHandler sets the handler for incoming content
func (m *Manager) SetContentHandler(handler types.ContentCallback) {
	m.handlerMutex.Lock()
	defer m.handlerMutex.Unlock()
	
	m.contentHandler = handler
	m.logger.Debug("Content handler set")
}

// JoinGroup joins a group
func (m *Manager) JoinGroup(group string) error {
	if !m.started {
		return fmt.Errorf("sync manager not started")
	}
	
	// Join the topic
	_, _, err := m.node.JoinTopic(group)
	if err != nil {
		return fmt.Errorf("failed to join group %s: %w", group, err)
	}
	
	// TODO: Set up subscription handling
	
	return nil
}

// LeaveGroup leaves a group
func (m *Manager) LeaveGroup(group string) error {
	if !m.started {
		return fmt.Errorf("sync manager not started")
	}
	
	// Leave the topic
	err := m.node.LeaveTopic(group)
	if err != nil {
		return fmt.Errorf("failed to leave group %s: %w", group, err)
	}
	
	return nil
}

// ListGroups lists the joined groups
func (m *Manager) ListGroups() ([]string, error) {
	if !m.started {
		return nil, fmt.Errorf("sync manager not started")
	}
	
	// Get the list of groups from topics
	groups := make([]string, 0, len(m.node.topics))
	for group := range m.node.topics {
		groups = append(groups, group)
	}
	
	return groups, nil
}

// GetDiscoveredPeers returns the list of discovered peers
func (m *Manager) GetDiscoveredPeers() []types.PeerInfo {
	// Get peers from the node
	internalPeers := m.node.GetPeers()
	
	// Convert to external peer info
	peers := make([]types.PeerInfo, 0, len(internalPeers))
	for _, peer := range internalPeers {
		peers = append(peers, peer.ToExternalPeerInfo())
	}
	
	return peers
}

// IsConnected checks if connected to the network
func (m *Manager) IsConnected() bool {
	if !m.started {
		return false
	}
	
	// Check if the node has any connections
	return len(m.node.Host().Network().Conns()) > 0
}

// AddPeer adds a peer by its multiaddress string
func (m *Manager) AddPeer(address string) error {
	if !m.started {
		return fmt.Errorf("sync manager not started")
	}
	
	return m.node.AddPeerByAddress(address)
}

// RemovePeer removes a peer by its ID
func (m *Manager) RemovePeer(peerID string) error {
	if !m.started {
		return fmt.Errorf("sync manager not started")
	}
	
	return m.node.RemovePeerByID(peerID)
}

// DisconnectPeer disconnects from a peer but keeps it in the peerstore
func (m *Manager) DisconnectPeer(peerID string) error {
	if !m.started {
		return fmt.Errorf("sync manager not started")
	}
	
	return m.node.DisconnectPeer(peerID)
}

// GetConnectedPeers returns a list of currently connected peers
func (m *Manager) GetConnectedPeers() []types.PeerInfo {
	if !m.started {
		return nil
	}
	
	connectedPeers := m.node.GetConnectedPeers()
	
	// Convert to external peer info
	result := make([]types.PeerInfo, 0, len(connectedPeers))
	for _, peer := range connectedPeers {
		result = append(result, peer.ToExternalPeerInfo())
	}
	
	return result
}

// EnablePairing enables pairing mode for the device
func (m *Manager) EnablePairing(handler types.PairingRequestCallback) (string, error) {
	if !m.started {
		return "", fmt.Errorf("sync manager not started")
	}
	
	// Wrap the callback to convert between types
	wrappedHandler := func(request PairingRequest, remotePeerID string) (bool, error) {
		// Convert to external type
		extRequest := types.PairingRequest{
			DeviceName: request.DeviceName,
			DeviceType: request.DeviceType,
			PeerID:     request.PeerID,
		}
		
		return handler(extRequest, remotePeerID)
	}
	
	return m.node.Pairing().EnablePairing(wrappedHandler)
}

// DisablePairing disables pairing mode
func (m *Manager) DisablePairing() {
	if !m.started {
		return
	}
	
	m.node.Pairing().DisablePairing()
}

// RequestPairing sends a pairing request to a device
func (m *Manager) RequestPairing(address string) (*types.PairingResponse, error) {
	if !m.started {
		return nil, fmt.Errorf("sync manager not started")
	}
	
	// Call the internal implementation
	internalResponse, err := m.node.Pairing().RequestPairing(address)
	if err != nil {
		return nil, err
	}
	
	// Convert to external type
	response := &types.PairingResponse{
		Accepted:     internalResponse.Accepted,
		ErrorMessage: internalResponse.ErrorMessage,
		PairingCode:  internalResponse.PairingCode,
		DeviceName:   internalResponse.DeviceName,
		DeviceType:   internalResponse.DeviceType,
		PeerID:       internalResponse.PeerID,
	}
	
	return response, nil
}

// IsPairingEnabled returns whether pairing is enabled
func (m *Manager) IsPairingEnabled() bool {
	if !m.started {
		return false
	}
	
	return m.node.Pairing().IsPairingEnabled()
}

// IsPaired checks if a device is paired
func (m *Manager) IsPaired(peerID string) bool {
	if !m.started {
		return false
	}
	
	return m.node.Pairing().IsPaired(peerID)
}

// GetPairedDevices returns all paired devices
func (m *Manager) GetPairedDevices() []types.PairedDevice {
	if !m.started {
		return nil
	}
	
	// Get internal paired devices
	internalDevices := m.node.Pairing().GetPairedDevices()
	
	// Convert to external type
	devices := make([]types.PairedDevice, 0, len(internalDevices))
	for _, device := range internalDevices {
		devices = append(devices, types.PairedDevice{
			PeerID:     device.PeerID,
			DeviceName: device.DeviceName,
			DeviceType: device.DeviceType,
			LastSeen:   device.LastSeen,
			PairedAt:   device.PairedAt,
		})
	}
	
	return devices
}

// RemovePairedDevice removes a paired device
func (m *Manager) RemovePairedDevice(peerID string) error {
	if !m.started {
		return fmt.Errorf("sync manager not started")
	}
	
	return m.node.Pairing().RemovePairedDevice(peerID)
}

// GetConfig returns the node's configuration
func (m *Manager) GetConfig() *SyncConfig {
	return m.config
}

// GetConfigFromGlobal retrieves sync configuration from the global config
func GetConfigFromGlobal(cfg *config.Config) *types.SyncConfig {
    // Map from the global config to our internal sync config
    syncCfg := &types.SyncConfig{
        // Core Sync Settings
		Enabled:           cfg.Sync.Enabled,
        SyncOverInternet:  cfg.Sync.SyncOverInternet,
        UseRelayNodes:     cfg.Sync.UseRelayNodes,
        ListenPort:        cfg.Sync.ListenPort,
        DiscoveryMethod:   cfg.Sync.DiscoveryMethod,

        // Clipboard Sync Options		
        ClipboardTypes:    cfg.Sync.ClipboardTypes,
        AutoCopyFromPeers: cfg.Sync.AutoCopyFromPeers,
        MaxClipboardSizeKB: cfg.Sync.MaxClipboardSizeKB,
        ClipboardHistorySize: cfg.Sync.ClipboardHistorySize,
        ClipboardBlacklistApps: cfg.Sync.ClipboardBlacklistApps,

        // File Transfer Options
        EnableFileSharing: cfg.Sync.EnableFileSharing,
        RequireFileConfirmation: cfg.Sync.RequireFileConfirmation,
        DefaultDownloadFolder: cfg.Sync.DefaultDownloadFolder,
        MaxFileSizeMB: cfg.Sync.MaxFileSizeMB,

        // Privacy & Security
        AllowOnlyKnownPeers: cfg.Sync.AllowOnlyKnownPeers,
        TrustedPeers: cfg.Sync.TrustedPeers,
        RequireApprovalPin: cfg.Sync.RequireApprovalPin,
        LogPeerActivity: cfg.Sync.LogPeerActivity,

        // Developer & Debug Options
        DebugLogging: cfg.Sync.DebugLogging,
        ShowPeerDebugInfo: cfg.Sync.ShowPeerDebugInfo,
        DisableMultiplexing: cfg.Sync.DisableMultiplexing,
        ForceDirectConnectionOnly: cfg.Sync.ForceDirectConnectionOnly,
    }
    
    return syncCfg
}