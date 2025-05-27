package discovery

import (
	"sync"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"go.uber.org/zap"
)

// MDNSDiscovery implements discovery using mDNS for local network discovery
type MDNSDiscovery struct {
	manager     *Manager
	host        host.Host
	service     mdns.Service
	serviceTag  string
	logger      *zap.Logger
	started     bool
	mutex       sync.RWMutex
}

// NewMDNSDiscovery creates a new mDNS discovery service
func NewMDNSDiscovery(manager *Manager, host host.Host, serviceTag string) *MDNSDiscovery {
	return &MDNSDiscovery{
		manager:    manager,
		host:       host,
		serviceTag: serviceTag,
		logger:     manager.logger.With(zap.String("service", "mdns")),
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
	
	// Set up mDNS discovery
	notifee := &mdnsNotifee{discovery: m}
	// Note: in newer versions of libp2p, NewMdnsService doesn't return an error
	service := mdns.NewMdnsService(m.host, m.serviceTag, notifee)
	
	m.service = service
	m.started = true
	m.logger.Info("Started mDNS discovery service")
	return nil
}

// Stop stops the mDNS discovery service
func (m *MDNSDiscovery) Stop() error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	
	if !m.started {
		return nil
	}
	
	if m.service != nil {
		m.service.Close()
		m.service = nil
	}
	
	m.started = false
	m.logger.Info("Stopped mDNS discovery service")
	return nil
}

// Name returns the name of the discovery service
func (m *MDNSDiscovery) Name() string {
	return "mdns"
}

// mdnsNotifee is a notifee for mDNS discoveries
type mdnsNotifee struct {
	discovery *MDNSDiscovery
}

// HandlePeerFound is called when a peer is discovered via mDNS
func (n *mdnsNotifee) HandlePeerFound(pi peer.AddrInfo) {
	// Skip our own peer ID
	if pi.ID == n.discovery.host.ID() {
		return
	}
	
	n.discovery.logger.Debug("Discovered peer via mDNS",
		zap.String("peer", pi.ID.String()),
		zap.Any("addrs", pi.Addrs))
	
	// Create peer info
	addrs := make([]string, 0, len(pi.Addrs))
	for _, addr := range pi.Addrs {
		addrs = append(addrs, addr.String())
	}
	
	peerInfo := types.DiscoveryPeerInfo{
		ID:         pi.ID.String(),
		Name:       "mDNS-Peer-" + pi.ID.ShortString(),
		Addrs:      addrs,
		DeviceType: "unknown",
		LastSeen:   time.Now(),
		Version:    "",
		Groups:     []string{},
		Capabilities: map[string]string{
			"discovery": "mdns",
		},
	}
	
	// Notify manager
	n.discovery.manager.HandlePeerDiscovered(peerInfo)
	
	// Try to connect to the peer
	go func() {
		ctx := n.discovery.manager.ctx
		if err := n.discovery.host.Connect(ctx, pi); err != nil {
			n.discovery.logger.Debug("Failed to connect to mDNS peer",
				zap.String("peer", pi.ID.String()),
				zap.Error(err))
		} else {
			n.discovery.logger.Debug("Connected to mDNS peer",
				zap.String("peer", pi.ID.String()))
		}
	}()
} 