package discovery

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/multiformats/go-multiaddr"
	"go.uber.org/zap"
)

// ManualDiscovery implements discovery using manually added peers
type ManualDiscovery struct {
	manager   *Manager
	host      host.Host
	logger    *zap.Logger
	started   bool
	mutex     sync.RWMutex
}

// NewManualDiscovery creates a new manual discovery service
func NewManualDiscovery(manager *Manager, host host.Host) *ManualDiscovery {
	return &ManualDiscovery{
		manager:  manager,
		host:     host,
		logger:   manager.logger.With(zap.String("service", "manual")),
		started:  false,
	}
}

// Start starts the manual discovery service
func (m *ManualDiscovery) Start() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	if m.started {
		return nil
	}
	
	m.started = true
	m.logger.Info("Started manual discovery service")
	return nil
}

// Stop stops the manual discovery service
func (m *ManualDiscovery) Stop() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	if !m.started {
		return nil
	}
	
	m.started = false
	m.logger.Info("Stopped manual discovery service")
	return nil
}

// Name returns the name of the discovery service
func (m *ManualDiscovery) Name() string {
	return "manual"
}

// AddPeer adds a peer by its multiaddress string
func (m *ManualDiscovery) AddPeer(addrStr string) error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	if !m.started {
		return fmt.Errorf("manual discovery service not started")
	}
	
	// Parse multiaddress
	addr, err := multiaddr.NewMultiaddr(addrStr)
	if err != nil {
		return fmt.Errorf("failed to parse multiaddress: %w", err)
	}
	
	// Extract peer ID and addresses
	info, err := peer.AddrInfoFromP2pAddr(addr)
	if err != nil {
		return fmt.Errorf("failed to extract peer info: %w", err)
	}
	
	// Make sure we're not trying to add ourselves
	if info.ID == m.host.ID() {
		return fmt.Errorf("cannot add self as peer")
	}
	
	// Try to connect to the peer
	ctx, cancel := context.WithTimeout(m.manager.ctx, 30*time.Second)
	defer cancel()
	
	if err := m.host.Connect(ctx, *info); err != nil {
		m.logger.Warn("Failed to connect to manually added peer",
			zap.String("peer", info.ID.String()),
			zap.Error(err))
		return fmt.Errorf("failed to connect to peer: %w", err)
	}
	
	m.logger.Info("Connected to manually added peer",
		zap.String("peer", info.ID.String()),
		zap.Any("addrs", info.Addrs))
	
	// Create peer info for notification
	addrs := make([]string, 0, len(info.Addrs))
	for _, addr := range info.Addrs {
		addrs = append(addrs, addr.String())
	}
	
	peerInfo := types.DiscoveryPeerInfo{
		ID:         info.ID.String(),
		Name:       "Manual-Peer-" + info.ID.ShortString(),
		Addrs:      addrs,
		DeviceType: "unknown",
		LastSeen:   time.Now(),
		Version:    "",
		Groups:     []string{},
		Capabilities: map[string]string{
			"discovery": "manual",
		},
	}
	
	// Notify manager
	m.manager.HandlePeerDiscovered(peerInfo)
	
	return nil
}

// RemovePeer removes a peer from the peerstore
func (m *ManualDiscovery) RemovePeer(peerID string) error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	if !m.started {
		return fmt.Errorf("manual discovery service not started")
	}
	
	// Parse peer ID
	id, err := peer.Decode(peerID)
	if err != nil {
		return fmt.Errorf("failed to parse peer ID: %w", err)
	}
	
	// // Create peer.AddrInfo with just the ID (no addresses)
	// info := peer.AddrInfo{
	// 	ID:    id,
	// 	Addrs: nil,
	// }
	
	// Remove peer from peerstore
	m.host.Peerstore().ClearAddrs(id)
	
	m.logger.Info("Removed peer",
		zap.String("peer", id.String()))
	
	return nil
} 