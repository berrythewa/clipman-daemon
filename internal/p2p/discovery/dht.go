package discovery

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"go.uber.org/zap"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
)

// DHTDiscovery implements discovery using a DHT for wider network discovery
type DHTDiscovery struct {
	manager         *Manager
	host            host.Host
	dht             *dht.IpfsDHT
	bootstrapPeers  []peer.AddrInfo
	ctx             context.Context
	cancel          context.CancelFunc
	logger          *zap.Logger
	started         bool
	discoveryTicker *time.Ticker
	mutex           sync.RWMutex
}

// NewDHTDiscovery creates a new DHT discovery service
func NewDHTDiscovery(manager *Manager, host host.Host, bootstrapPeers []string) (*DHTDiscovery, error) {
	// Convert bootstrap peer strings to AddrInfo
	var peerInfos []peer.AddrInfo
	for _, addrStr := range bootstrapPeers {
		addr, err := peer.AddrInfoFromString(addrStr)
		if err != nil {
			manager.logger.Warn("Failed to parse bootstrap peer address",
				zap.String("addr", addrStr),
				zap.Error(err))
			continue
		}
		peerInfos = append(peerInfos, *addr)
	}
	
	discoveryCtx, cancel := context.WithCancel(manager.ctx)
	
	return &DHTDiscovery{
		manager:        manager,
		host:           host,
		bootstrapPeers: peerInfos,
		ctx:            discoveryCtx,
		cancel:         cancel,
		logger:         manager.logger.With(zap.String("service", "dht")),
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
	
	// Create DHT options with simplified approach
	dhtOpts := []dht.Option{
		dht.Mode(dht.ModeClient),
		dht.ProtocolPrefix("/clipman"),  // Use a custom protocol prefix to avoid collisions
		// Note: Removed problematic dht.Validator option
	}
	
	// Create the DHT
	kadDHT, err := dht.New(d.ctx, d.host, dhtOpts...)
	if err != nil {
		return fmt.Errorf("failed to create DHT: %w", err)
	}
	d.dht = kadDHT
	
	// Connect to bootstrap peers if available
	if len(d.bootstrapPeers) > 0 {
		d.logger.Info("Connecting to bootstrap peers", zap.Int("count", len(d.bootstrapPeers)))
		
		// Connect to bootstrap peers in parallel
		var wg sync.WaitGroup
		for _, peerInfo := range d.bootstrapPeers {
			wg.Add(1)
			go func(pi peer.AddrInfo) {
				defer wg.Done()
				
				// Create a timeout context for this connection attempt
				ctx, cancel := context.WithTimeout(d.ctx, 20*time.Second)
				defer cancel()
				
				d.logger.Debug("Connecting to bootstrap peer", zap.String("peer", pi.ID.String()))
				if err := d.host.Connect(ctx, pi); err != nil {
					d.logger.Warn("Failed to connect to bootstrap peer", 
						zap.String("peer", pi.ID.String()),
						zap.Error(err))
					return
				}
				d.logger.Debug("Connected to bootstrap peer", zap.String("peer", pi.ID.String()))
			}(peerInfo)
		}
		
		// Wait for all connection attempts to complete (or timeout)
		wg.Wait()
	} else {
		d.logger.Warn("No bootstrap peers configured for DHT")
	}
	
	// Bootstrap the DHT
	if err := d.dht.Bootstrap(d.ctx); err != nil {
		d.logger.Error("Failed to bootstrap DHT", zap.Error(err))
		// Continue anyway, might still work with manually connected peers
	}
	
	// Start discovery ticker
	d.discoveryTicker = time.NewTicker(5 * time.Minute)
	go d.discoverPeersLoop()
	
	d.started = true
	d.logger.Info("Started DHT discovery service")
	return nil
}

// Stop stops the DHT discovery service
func (d *DHTDiscovery) Stop() error {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	if !d.started {
		return nil
	}
	
	// Stop the ticker
	if d.discoveryTicker != nil {
		d.discoveryTicker.Stop()
		d.discoveryTicker = nil
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
	d.logger.Info("DHT discovery service stopped")
	return nil
}

// Name returns the name of the discovery service
func (d *DHTDiscovery) Name() string {
	return "dht"
}

// discoverPeersLoop runs a continuous loop to discover peers
func (d *DHTDiscovery) discoverPeersLoop() {
	for {
		select {
		case <-d.ctx.Done():
			return
		case <-d.discoveryTicker.C:
			d.discoverDHTPeers()
		}
	}
}

// discoverDHTPeers performs DHT-based peer discovery
func (d *DHTDiscovery) discoverDHTPeers() {
	d.mutex.RLock()
	if !d.started || d.dht == nil {
		d.mutex.RUnlock()
		return
	}
	d.mutex.RUnlock()
	
	d.logger.Debug("Discovering peers via DHT")
	
	// Use basic provider discovery
	ctx, cancel := context.WithTimeout(d.ctx, 30*time.Second)
	defer cancel()
	
	// Use a consistent key for discovery
	rendezvousKey := "/clipman/peers/v1"
	
	// Look for peers on the DHT
	providers := d.dht.FindProvidersAsync(ctx, convertStringToCID(rendezvousKey), 100)
	
	// Process found providers
	for provider := range providers {
		// Skip self
		if provider.ID == d.host.ID() {
			continue
		}
		
		d.logger.Debug("DHT: Found peer", zap.String("peer", provider.ID.String()))
		
		// Convert to DiscoveryPeerInfo
		addrs := make([]string, 0, len(provider.Addrs))
		for _, addr := range provider.Addrs {
			addrs = append(addrs, addr.String())
		}
		
		peerInfo := types.DiscoveryPeerInfo{
			ID:         provider.ID.String(),
			Name:       "DHT-Peer-" + provider.ID.ShortString(),
			Addrs:      addrs,
			DeviceType: "unknown",
			LastSeen:   time.Now(),
			Version:    "",
			Groups:     []string{},
			Capabilities: map[string]string{
				"discovery": "dht",
			},
		}
		
		// Notify manager
		d.manager.HandlePeerDiscovered(peerInfo)
		
		// Try to connect
		go func(pi peer.AddrInfo) {
			ctx, cancel := context.WithTimeout(d.ctx, 15*time.Second)
			defer cancel()
			
			if err := d.host.Connect(ctx, pi); err != nil {
				d.logger.Debug("Failed to connect to DHT peer", 
					zap.String("peer", pi.ID.String()),
					zap.Error(err))
			} else {
				d.logger.Debug("Connected to DHT peer", 
					zap.String("peer", pi.ID.String()))
			}
		}(provider)
	}
}

// Utility function to convert a string to a CID for DHT operations
func convertStringToCID(s string) cid.Cid {
	// Create a multihash from the string
	h, err := multihash.Sum([]byte(s), multihash.SHA2_256, -1)
	if err != nil {
		// This should never happen with SHA2_256
		panic(fmt.Sprintf("failed to create multihash: %v", err))
	}
	
	// Create a CID from the multihash
	return cid.NewCidV1(cid.Raw, h)
} 