package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/libp2p/go-libp2p/core/host"
	"go.uber.org/zap"
)

// PeerDiscoveredCallback is a callback for when a peer is discovered
type PeerDiscoveredCallback func(types.DiscoveryPeerInfo)

// Manager manages multiple discovery services
type Manager struct {
	// Services
	services map[string]Service
	
	// Host and context
	host         host.Host
	ctx          context.Context
	cancel       context.CancelFunc
	logger       *zap.Logger
	config       *Config
	
	// Callbacks
	onPeerDiscovered PeerDiscoveredCallback
	
	// State
	started      bool
	mutex        sync.RWMutex
}

// NewManager creates a new discovery manager
func NewManager(ctx context.Context, host host.Host, config *Config, logger *zap.Logger) *Manager {
	discoveryCtx, cancel := context.WithCancel(ctx)
	
	dm := &Manager{
		services:     make(map[string]Service),
		host:         host,
		ctx:          discoveryCtx,
		cancel:       cancel,
		logger:       logger.With(zap.String("component", "discovery-manager")),
		config:       config,
		started:      false,
	}
	
	// Add mDNS service if enabled
	if config.EnableMDNS {
		dm.AddService("mdns", NewMDNSDiscovery(dm, host, "clipman"))
	}
	
	// Add DHT service if enabled
	if config.EnableDHT {
		dhtService, err := NewDHTDiscovery(dm, host, config.BootstrapPeers)
		if err != nil {
			logger.Warn("Failed to create DHT discovery service", zap.Error(err))
		} else {
			dm.AddService("dht", dhtService)
		}
	}
	
	// Always add manual discovery (for direct peer connections)
	dm.AddService("manual", NewManualDiscovery(dm, host))
	
	return dm
}

// AddService adds a discovery service
func (dm *Manager) AddService(name string, service Service) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()
	
	dm.services[name] = service
	dm.logger.Debug("Added discovery service", zap.String("service", name))
}

// Start starts all discovery services
func (dm *Manager) Start() error {
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
func (dm *Manager) Stop() error {
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
func (dm *Manager) SetPeerDiscoveredCallback(callback PeerDiscoveredCallback) {
	dm.mutex.Lock()
	defer dm.mutex.Unlock()
	
	dm.onPeerDiscovered = callback
}

// HandlePeerDiscovered handles discovered peers
func (dm *Manager) HandlePeerDiscovered(peerInfo types.DiscoveryPeerInfo) {
	dm.mutex.RLock()
	callback := dm.onPeerDiscovered
	dm.mutex.RUnlock()
	
	if callback != nil {
		callback(peerInfo)
	}
}

// GetManualDiscovery returns the manual discovery service if available
func (dm *Manager) GetManualDiscovery() (*ManualDiscovery, error) {
	dm.mutex.RLock()
	defer dm.mutex.RUnlock()
	
	service, ok := dm.services["manual"]
	if !ok {
		return nil, fmt.Errorf("manual discovery service not available")
	}
	
	// Type assertion
	manualService, ok := service.(*ManualDiscovery)
	if !ok {
		return nil, fmt.Errorf("manual discovery service has unexpected type")
	}
	
	return manualService, nil
}

// SaveDiscoveredPeers saves the list of known peers to a file for persistence
func (dm *Manager) SaveDiscoveredPeers(peers map[string]types.DiscoveryPeerInfo) error {
	// Skip if no peers to save
	if len(peers) == 0 {
		return nil
	}
	
	// Skip if persistence is disabled
	if !dm.config.PersistPeers {
		return nil
	}
	
	// Ensure directory exists
	dir := filepath.Dir(dm.config.PeersPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create peers directory: %w", err)
	}
	
	// Convert to JSON
	data, err := json.MarshalIndent(peers, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal peers: %w", err)
	}
	
	// Write to file
	if err := os.WriteFile(dm.config.PeersPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write peers file: %w", err)
	}
	
	dm.logger.Debug("Saved discovered peers", zap.Int("count", len(peers)))
	return nil
}

// LoadDiscoveredPeers loads the list of known peers from a file
func (dm *Manager) LoadDiscoveredPeers() (map[string]types.DiscoveryPeerInfo, error) {
	// Skip if persistence is disabled
	if !dm.config.PersistPeers {
		return nil, nil
	}
	
	// Check if file exists
	if _, err := os.Stat(dm.config.PeersPath); os.IsNotExist(err) {
		return nil, nil
	}
	
	// Read file
	data, err := os.ReadFile(dm.config.PeersPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read peers file: %w", err)
	}
	
	// Parse JSON
	var peers map[string]types.DiscoveryPeerInfo
	if err := json.Unmarshal(data, &peers); err != nil {
		return nil, fmt.Errorf("failed to unmarshal peers: %w", err)
	}
	
	dm.logger.Debug("Loaded discovered peers", zap.Int("count", len(peers)))
	return peers, nil
} 