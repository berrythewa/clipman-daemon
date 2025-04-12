// Package sync provides clipboard synchronization using libp2p
package sync

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"go.uber.org/zap"
)

// DiscoveryService represents a method for discovering peers
type DiscoveryService interface {
	// Start starts the discovery service
	Start() error
	
	// Stop stops the discovery service
	Stop() error
	
	// Name returns the name of the discovery service
	Name() string
}

// PeerDiscoveredCallback is called when a peer is discovered
type PeerDiscoveredCallback func(peerInfo InternalPeerInfo)

// DiscoveryManager manages multiple discovery services
type DiscoveryManager struct {
	// Services
	services map[string]DiscoveryService
	
	// Host and context
	host         host.Host
	ctx          context.Context
	cancel       context.CancelFunc
	logger       *zap.Logger
	
	// Callbacks
	onPeerDiscovered PeerDiscoveredCallback
	
	// State
	started      bool
	mutex        sync.RWMutex
}

// NewDiscoveryManager creates a new discovery manager
func NewDiscoveryManager(ctx context.Context, host host.Host, config *SyncConfig, logger *zap.Logger) *DiscoveryManager {
	discoveryCtx, cancel := context.WithCancel(ctx)
	
	dm := &DiscoveryManager{
		services:     make(map[string]DiscoveryService),
		host:         host,
		ctx:          discoveryCtx,
		cancel:       cancel,
		logger:       logger.With(zap.String("component", "discovery-manager")),
		started:      false,
	}
	
	// Get discovery config
	discCfg := GetDiscoveryConfig(config)
	
	// Add mDNS service if enabled
	if discCfg.EnableMDNS {
		dm.AddService("mdns", NewMDNSDiscovery(dm, host, "clipman"))
	}
	
	// Add DHT service if enabled
	if discCfg.EnableDHT {
		dhtService, err := NewDHTDiscovery(dm, host, discCfg.BootstrapPeers)
		if err != nil {
			logger.Warn("Failed to create DHT discovery service", zap.Error(err))
		} else {
			dm.AddService("dht", dhtService)
		}
	}
	
	return dm
}

// AddService adds a discovery service
func (dm *DiscoveryManager) AddService(name string, service DiscoveryService) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()
	
	dm.services[name] = service
	dm.logger.Debug("Added discovery service", zap.String("service", name))
}

// Start starts all discovery services
func (dm *DiscoveryManager) Start() error {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()
	
	if dm.started {
		return nil
	}
	
	for name, service := range dm.services {
		if err := service.Start(); err != nil {
			dm.logger.Error("Failed to start discovery service", 
				zap.String("service", name),
				zap.Error(err))
			continue
		}
		dm.logger.Info("Started discovery service", zap.String("service", name))
	}
	
	dm.started = true
	return nil
}

// Stop stops all discovery services
func (dm *DiscoveryManager) Stop() error {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()
	
	if !dm.started {
		return nil
	}
	
	for name, service := range dm.services {
		if err := service.Stop(); err != nil {
			dm.logger.Error("Failed to stop discovery service", 
				zap.String("service", name),
				zap.Error(err))
			continue
		}
		dm.logger.Debug("Stopped discovery service", zap.String("service", name))
	}
	
	dm.cancel()
	dm.started = false
	return nil
}

// SetPeerDiscoveredCallback sets the callback for when a peer is discovered
func (dm *DiscoveryManager) SetPeerDiscoveredCallback(callback PeerDiscoveredCallback) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()
	
	dm.onPeerDiscovered = callback
}

// HandlePeerDiscovered handles discovered peers
func (dm *DiscoveryManager) HandlePeerDiscovered(peerInfo InternalPeerInfo) {
	dm.mutex.RLock()
	callback := dm.onPeerDiscovered
	dm.mutex.RUnlock()
	
	if callback != nil {
		callback(peerInfo)
	}
}

// MDNSDiscovery provides discovery via mDNS
type MDNSDiscovery struct {
	manager     *DiscoveryManager
	host        host.Host
	service     mdns.Service
	serviceTag  string
	logger      *zap.Logger
	started     bool
	mutex       sync.RWMutex
}

// NewMDNSDiscovery creates a new mDNS discovery service
func NewMDNSDiscovery(manager *DiscoveryManager, host host.Host, serviceTag string) *MDNSDiscovery {
	return &MDNSDiscovery{
		manager:    manager,
		host:       host,
		serviceTag: serviceTag,
		logger:     manager.logger.With(zap.String("discovery", "mdns")),
		started:    false,
	}
}

// Start starts the mDNS discovery service
func (m *MDNSDiscovery) Start() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	if m.started {
		return nil
	}
	
	// Create a new mDNS service
	service := mdns.NewMdnsService(m.host, m.serviceTag, &mdnsNotifee{discovery: m})
	if err := service.Start(); err != nil {
		return fmt.Errorf("failed to start mDNS service: %w", err)
	}
	
	m.service = service
	m.started = true
	m.logger.Info("Started mDNS discovery", zap.String("service_tag", m.serviceTag))
	return nil
}

// Stop stops the mDNS discovery service
func (m *MDNSDiscovery) Stop() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	if !m.started || m.service == nil {
		return nil
	}
	
	m.service.Close()
	m.service = nil
	m.started = false
	m.logger.Info("Stopped mDNS discovery")
	return nil
}

// Name returns the name of the discovery service
func (m *MDNSDiscovery) Name() string {
	return "mdns"
}

// mdnsNotifee handles peer discovery via mDNS
type mdnsNotifee struct {
	discovery *MDNSDiscovery
}

// HandlePeerFound is called when a peer is discovered via mDNS
func (n *mdnsNotifee) HandlePeerFound(pi peer.AddrInfo) {
	// Ignore our own peer ID
	if pi.ID == n.discovery.host.ID() {
		return
	}
	
	n.discovery.logger.Debug("Discovered peer via mDNS", zap.String("peer_id", pi.ID.String()))
	
	// Add peer to host peerstore
	n.discovery.host.Peerstore().AddAddrs(pi.ID, pi.Addrs, time.Hour)
	
	// Create peer info
	peerInfo := InternalPeerInfo{
		ID:           pi.ID.String(),
		LastSeen:     time.Now(),
		Addrs:        make([]string, 0, len(pi.Addrs)),
		Capabilities: make(map[string]string),
		DeviceType:   "unknown", // Will be determined through protocol
	}
	
	// Convert addresses to strings
	for _, addr := range pi.Addrs {
		peerInfo.Addrs = append(peerInfo.Addrs, addr.String())
	}
	
	// Notify the discovery manager
	n.discovery.manager.HandlePeerDiscovered(peerInfo)
}

// DHTDiscovery provides discovery via DHT
type DHTDiscovery struct {
	manager       *DiscoveryManager
	host          host.Host
	dht           *dht.IpfsDHT
	bootstrapPeers []peer.AddrInfo
	ctx           context.Context
	cancel        context.CancelFunc
	logger        *zap.Logger
	started       bool
	mutex         sync.RWMutex
}

// NewDHTDiscovery creates a new DHT discovery service
func NewDHTDiscovery(manager *DiscoveryManager, host host.Host, bootstrapPeers []string) (*DHTDiscovery, error) {
	// Parse bootstrap peers
	var bootPeers []peer.AddrInfo
	
	for _, peerAddr := range bootstrapPeers {
		addr, err := peer.AddrInfoFromString(peerAddr)
		if err != nil {
			manager.logger.Warn("Failed to parse bootstrap peer", 
				zap.String("peer", peerAddr),
				zap.Error(err))
			continue
		}
		bootPeers = append(bootPeers, *addr)
	}
	
	ctx, cancel := context.WithCancel(manager.ctx)
	
	return &DHTDiscovery{
		manager:        manager,
		host:           host,
		bootstrapPeers: bootPeers,
		ctx:            ctx,
		cancel:         cancel,
		logger:         manager.logger.With(zap.String("discovery", "dht")),
		started:        false,
	}, nil
}

// Start starts the DHT discovery service
func (d *DHTDiscovery) Start() error {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	if d.started {
		return nil
	}
	
	// Create a new DHT (currently a placeholder - actual implementation TBD)
	d.logger.Warn("DHT discovery not fully implemented yet")
	
	// Mark as started (even though not fully implemented)
	d.started = true
	d.logger.Info("Started DHT discovery (placeholder)")
	return nil
}

// Stop stops the DHT discovery service
func (d *DHTDiscovery) Stop() error {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	if !d.started {
		return nil
	}
	
	// Cancel the context
	d.cancel()
	
	// Close the DHT if it exists
	if d.dht != nil {
		if err := d.dht.Close(); err != nil {
			d.logger.Warn("Error closing DHT", zap.Error(err))
		}
		d.dht = nil
	}
	
	d.started = false
	d.logger.Info("Stopped DHT discovery")
	return nil
}

// Name returns the name of the discovery service
func (d *DHTDiscovery) Name() string {
	return "dht"
} 