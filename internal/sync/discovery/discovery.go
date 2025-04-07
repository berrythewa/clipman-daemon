// Package discovery provides peer discovery mechanisms for the sync package
package discovery

import (
	"fmt"
	"sync"

	"github.com/berrythewa/clipman-daemon/internal/sync"
	"go.uber.org/zap"
)

// DiscoveryMethod represents a method for peer discovery
type DiscoveryMethod string

// Available discovery methods
const (
	MethodMDNS DiscoveryMethod = "mdns"     // Multicast DNS-based discovery (local network)
	MethodMQTT DiscoveryMethod = "mqtt"     // MQTT-based discovery (through MQTT broker)
	MethodDHT  DiscoveryMethod = "dht"      // DHT-based discovery (P2P networks)
	MethodWeb  DiscoveryMethod = "web"      // Web-based discovery (through HTTP endpoints)
)

// PeerDiscoveryHandler is called when a peer is discovered or lost
type PeerDiscoveryHandler func(peer sync.PeerInfo, discovered bool)

// DiscoveryFactory creates discovery mechanisms
type DiscoveryFactory interface {
	// CreateDiscovery creates a discovery mechanism with the given options
	CreateDiscovery(options interface{}) (sync.Discovery, error)
	
	// SupportsConfig checks if this factory supports the given configuration
	SupportsConfig(config interface{}) bool
}

// DiscoveryOptions contains common options for discovery mechanisms
type DiscoveryOptions struct {
	AnnounceInterval int           // How often to announce presence (in seconds)
	PeerTimeout      int           // How long until a peer is considered offline (in seconds)
	DeviceID         string        // Device ID for this peer
	DeviceName       string        // Human-readable name for this peer
	Logger           *zap.Logger   // Logger for discovery
}

// Global registry of discovery factories
var (
	discoveryFactories = make(map[string]DiscoveryFactory)
	factoryMutex       sync.RWMutex
)

// RegisterDiscoveryFactory registers a discovery factory
func RegisterDiscoveryFactory(name string, factory DiscoveryFactory) {
	factoryMutex.Lock()
	defer factoryMutex.Unlock()

	discoveryFactories[name] = factory
}

// GetDiscoveryFactory returns a discovery factory by name
func GetDiscoveryFactory(name string) (DiscoveryFactory, error) {
	factoryMutex.RLock()
	defer factoryMutex.RUnlock()

	factory, ok := discoveryFactories[name]
	if !ok {
		return nil, fmt.Errorf("discovery factory not found: %s", name)
	}

	return factory, nil
}

// BaseDiscovery provides common functionality for discovery implementations
type BaseDiscovery struct {
	options    *DiscoveryOptions
	running    bool
	peers      map[string]sync.PeerInfo
	handlers   []PeerDiscoveryHandler
	mutex      sync.RWMutex
	logger     *zap.Logger
}

// NewBaseDiscovery creates a new base discovery
func NewBaseDiscovery(options *DiscoveryOptions) *BaseDiscovery {
	// Use a default logger if none provided
	logger := options.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	
	return &BaseDiscovery{
		options:  options,
		running:  false,
		peers:    make(map[string]sync.PeerInfo),
		handlers: make([]PeerDiscoveryHandler, 0),
		logger:   logger.With(zap.String("component", "discovery")),
	}
}

// SetPeerDiscoveryHandler sets the handler for peer discovery events
func (d *BaseDiscovery) SetPeerDiscoveryHandler(handler PeerDiscoveryHandler) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	d.handlers = []PeerDiscoveryHandler{handler}
	d.logger.Debug("Peer discovery handler set")
}

// AddPeerDiscoveryHandler adds a handler for peer discovery events
func (d *BaseDiscovery) AddPeerDiscoveryHandler(handler PeerDiscoveryHandler) {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	d.handlers = append(d.handlers, handler)
	d.logger.Debug("Peer discovery handler added", 
		zap.Int("total_handlers", len(d.handlers)))
}

// GetDiscoveredPeers returns the list of discovered peers
func (d *BaseDiscovery) GetDiscoveredPeers() []sync.PeerInfo {
	d.mutex.RLock()
	defer d.mutex.RUnlock()
	
	peers := make([]sync.PeerInfo, 0, len(d.peers))
	for _, peer := range d.peers {
		peers = append(peers, peer)
	}
	
	return peers
}

// notifyPeerDiscovered notifies handlers that a peer was discovered
func (d *BaseDiscovery) notifyPeerDiscovered(peer sync.PeerInfo) {
	d.mutex.RLock()
	handlers := make([]PeerDiscoveryHandler, len(d.handlers))
	copy(handlers, d.handlers)
	d.mutex.RUnlock()
	
	d.logger.Debug("Peer discovered", 
		zap.String("peer_id", peer.ID),
		zap.String("peer_name", peer.Name))
	
	for _, handler := range handlers {
		if handler != nil {
			go handler(peer, true)
		}
	}
}

// notifyPeerLost notifies handlers that a peer was lost
func (d *BaseDiscovery) notifyPeerLost(peer sync.PeerInfo) {
	d.mutex.RLock()
	handlers := make([]PeerDiscoveryHandler, len(d.handlers))
	copy(handlers, d.handlers)
	d.mutex.RUnlock()
	
	d.logger.Debug("Peer lost", 
		zap.String("peer_id", peer.ID),
		zap.String("peer_name", peer.Name))
	
	for _, handler := range handlers {
		if handler != nil {
			go handler(peer, false)
		}
	}
}

// addOrUpdatePeer adds or updates a peer in the discovered peers list
func (d *BaseDiscovery) addOrUpdatePeer(peer sync.PeerInfo) bool {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	existing, found := d.peers[peer.ID]
	d.peers[peer.ID] = peer
	
	// Return true if this is a new peer
	return !found || existing.LastSeen.Before(peer.LastSeen)
}

// removePeer removes a peer from the discovered peers list
func (d *BaseDiscovery) removePeer(peerID string) bool {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	_, found := d.peers[peerID]
	if found {
		delete(d.peers, peerID)
	}
	
	return found
} 