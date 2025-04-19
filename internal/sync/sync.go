// Package sync provides clipboard synchronization using libp2p
package sync

import (
	"context"
	"fmt"
	"sync"

	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/sync/discovery"
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

// EnablePairing enables pairing mode
func (m *Manager) EnablePairing(handler types.PairingRequestCallback) (string, error) {
	if !m.started {
		return "", fmt.Errorf("sync manager not started")
	}
	
	// Create a wrapper that adapts the types.PairingRequestCallback to our internal PairingRequestCallback
	internalHandler := func(request PairingRequest, remotePeerId string) (bool, error) {
		// Convert our internal PairingRequest to types.PairingRequest
		externalRequest := types.PairingRequest{
			DeviceName: request.DeviceName,
			DeviceType: request.DeviceType,
			PeerID:     request.PeerID,
		}
		
		// Call the handler
		return handler(externalRequest, remotePeerId)
	}
	
	// Enable pairing mode on the node
	return m.node.pairing.EnablePairing(internalHandler)
}

// DisablePairing disables pairing mode
func (m *Manager) DisablePairing() {
	if !m.started {
		return
	}
	
	m.node.pairing.DisablePairing()
}

// RequestPairing requests pairing with a device at the given address
func (m *Manager) RequestPairing(address string) (*types.PairingResponse, error) {
	if !m.started {
		return nil, fmt.Errorf("sync manager not started")
	}
	
	// First, try to establish a connection with the peer
	if err := m.node.AddPeerByAddress(address); err != nil {
		return nil, fmt.Errorf("failed to connect to peer: %w", err)
	}
	
	// Request pairing from the node's pairing manager
	internalResponse, err := m.node.pairing.RequestPairing(address)
	if err != nil {
		return nil, err
	}
	
	// Convert to external type
	externalResponse := &types.PairingResponse{
		Accepted:     internalResponse.Accepted,
		ErrorMessage: internalResponse.ErrorMessage,
		PairingCode:  internalResponse.PairingCode,
		DeviceName:   internalResponse.DeviceName,
		DeviceType:   internalResponse.DeviceType,
		PeerID:       internalResponse.PeerID,
	}
	
	return externalResponse, nil
}

// IsPairingEnabled checks if pairing mode is enabled
func (m *Manager) IsPairingEnabled() bool {
	if !m.started {
		return false
	}
	return m.node.pairing.IsPairingEnabled()
}

// IsPaired checks if a peer is paired
func (m *Manager) IsPaired(peerID string) bool {
	if !m.started {
		return false
	}
	return m.node.pairing.IsPaired(peerID)
}

// GetPairedDevices returns a list of paired devices
func (m *Manager) GetPairedDevices() []types.PairedDevice {
	if !m.started {
		return nil
	}
	
	// Get devices from the pairing manager
	internalDevices := m.node.pairing.GetPairedDevices()
	
	// Convert to the external type
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
	
	return m.node.pairing.RemovePairedDevice(peerID)
}

// GetConfig returns the sync configuration for application use
func (m *Manager) GetConfig() *types.SyncConfig {
	if m.config == nil {
		return nil
	}
	
	return &types.SyncConfig{
		Enabled:           m.config.Enabled,
		SyncOverInternet:  m.config.SyncOverInternet,
		UseRelayNodes:     m.config.UseRelayNodes,
		ListenPort:        m.config.ListenPort,
		DiscoveryMethod:   m.config.DiscoveryMethod,
		
		ClipboardTypes:         m.config.ClipboardTypes,
		AutoCopyFromPeers:      m.config.AutoCopyFromPeers,
		MaxClipboardSizeKB:     m.config.MaxClipboardSizeKB,
		ClipboardHistorySize:   m.config.ClipboardHistorySize,
		ClipboardBlacklistApps: m.config.ClipboardBlacklistApps,
		
		EnableFileSharing:       m.config.EnableFileSharing,
		RequireFileConfirmation: m.config.RequireFileConfirmation,
		DefaultDownloadFolder:   m.config.DefaultDownloadFolder,
		MaxFileSizeMB:           m.config.MaxFileSizeMB,
		
		AllowOnlyKnownPeers: m.config.AllowOnlyKnownPeers,
		TrustedPeers:        m.config.TrustedPeers,
		RequireApprovalPin:  m.config.RequireApprovalPin,
		LogPeerActivity:     m.config.LogPeerActivity,
		
		DebugLogging:              m.config.DebugLogging,
		ShowPeerDebugInfo:         m.config.ShowPeerDebugInfo,
		DisableMultiplexing:       m.config.DisableMultiplexing,
		ForceDirectConnectionOnly: m.config.ForceDirectConnectionOnly,
	}
}

// GetConfigFromGlobal extracts sync configuration from the global config
func GetConfigFromGlobal(cfg *config.Config) *types.SyncConfig {
	if cfg == nil {
		return nil
	}
	
	// Map from global config to our external type
	syncCfg := &types.SyncConfig{
		Enabled:          cfg.Sync.Enabled,
		SyncOverInternet: cfg.Sync.SyncOverInternet,
		UseRelayNodes:    cfg.Sync.UseRelayNodes,
		ListenPort:       cfg.Sync.ListenPort,
		DiscoveryMethod:  cfg.Sync.DiscoveryMethod,
		
		// Add other fields from config
		AllowOnlyKnownPeers: cfg.Sync.AllowOnlyKnownPeers,
		RequireApprovalPin:  cfg.Sync.RequireApprovalPin,
	}
	
	return syncCfg
}