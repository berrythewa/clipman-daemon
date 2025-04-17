// Package sync provides clipboard synchronization using libp2p
package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/sync/discovery"
	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	"go.uber.org/zap"
	"github.com/ipfs/go-cid"
	"github.com/multiformats/go-multihash"
	"github.com/multiformats/go-multiaddr"
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
	config       *SyncConfig
	
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
		config:       config,
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
	
	// Always add manual discovery (for direct peer connections)
	dm.AddService("manual", NewManualDiscovery(dm, host))
	
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

// GetManualDiscovery returns the manual discovery service if available
func (dm *DiscoveryManager) GetManualDiscovery() (*ManualDiscovery, error) {
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
func (dm *DiscoveryManager) SaveDiscoveredPeers(peers map[string]InternalPeerInfo) error {
	// Skip if no peers to save
	if len(peers) == 0 {
		return nil
	}
	
	// Skip if persistence is disabled
	if !dm.config.PersistDiscoveredPeers {
		return nil
	}
	
	// Create the storage path if it doesn't exist
	storagePath := ExpandPath(dm.config.DiscoveredPeersPath)
	dirPath := filepath.Dir(storagePath)
	
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return fmt.Errorf("failed to create directory for peers storage: %w", err)
	}
	
	// Convert the map to a slice for serialization
	peersList := make([]InternalPeerInfo, 0, len(peers))
	for _, peer := range peers {
		// We only want to save peers that have addresses
		if len(peer.Addrs) > 0 {
			peersList = append(peersList, peer)
		}
	}
	
	// Marshal the peers to JSON
	data, err := json.MarshalIndent(peersList, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal peers data: %w", err)
	}
	
	// Write to file
	if err := os.WriteFile(storagePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write peers data to file: %w", err)
	}
	
	dm.logger.Debug("Saved discovered peers", zap.Int("count", len(peersList)), zap.String("path", storagePath))
	return nil
}

// LoadDiscoveredPeers loads previously saved peers from a file
func (dm *DiscoveryManager) LoadDiscoveredPeers() (map[string]InternalPeerInfo, error) {
	result := make(map[string]InternalPeerInfo)
	
	// Skip if persistence is disabled
	if !dm.config.PersistDiscoveredPeers {
		return result, nil
	}
	
	// Check if the file exists
	storagePath := ExpandPath(dm.config.DiscoveredPeersPath)
	if _, err := os.Stat(storagePath); os.IsNotExist(err) {
		// File doesn't exist yet, that's fine
		return result, nil
	}
	
	// Read the file
	data, err := os.ReadFile(storagePath)
	if err != nil {
		return result, fmt.Errorf("failed to read peers data from file: %w", err)
	}
	
	// Unmarshal the data
	var peersList []InternalPeerInfo
	if err := json.Unmarshal(data, &peersList); err != nil {
		return result, fmt.Errorf("failed to unmarshal peers data: %w", err)
	}
	
	// Convert to map
	for _, peer := range peersList {
		result[peer.ID] = peer
	}
	
	dm.logger.Debug("Loaded discovered peers", zap.Int("count", len(result)), zap.String("path", storagePath))
	return result, nil
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
	manager         *DiscoveryManager
	host            host.Host
	dht             *dht.IpfsDHT
	rendezvous      *RendezvousDHT
	bootstrapPeers  []peer.AddrInfo
	ctx             context.Context
	cancel          context.CancelFunc
	logger          *zap.Logger
	started         bool
	discoveryTicker *time.Ticker
	mutex           sync.RWMutex
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
		manager:         manager,
		host:            host,
		bootstrapPeers:  bootPeers,
		ctx:             ctx,
		cancel:          cancel,
		logger:          manager.logger.With(zap.String("discovery", "dht")),
		started:         false,
	}, nil
}

// Start starts the DHT discovery service
func (d *DHTDiscovery) Start() error {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	if d.started {
		return nil
	}
	
	// Get the sync config to access DHT-specific options
	syncConfig := d.manager.config
	
	// Determine DHT mode
	var dhtMode dht.ModeOpt
	if syncConfig.DHTServerMode {
		d.logger.Info("Running DHT in server mode")
		dhtMode = dht.ModeServer
	} else {
		d.logger.Info("Running DHT in client mode")
		dhtMode = dht.ModeClient
	}
	
	// Build DHT options
	dhtOpts := []dht.Option{
		dht.Mode(dhtMode),
		dht.ProtocolPrefix("/clipman"),  // Use a custom protocol prefix to avoid collisions
		dht.Validator(dht.DefaultValidator{}),
	}
	
	// Configure persistent storage if enabled
	if syncConfig.DHTPersistentStorage {
		// Expand the path (e.g., resolve ~ to home directory)
		storagePath := ExpandPath(syncConfig.DHTStoragePath)
		
		// Create the directory if it doesn't exist
		if err := os.MkdirAll(storagePath, 0755); err != nil {
			d.logger.Warn("Failed to create DHT storage directory", 
				zap.String("path", storagePath), 
				zap.Error(err))
		} else {
			d.logger.Info("Using persistent storage for DHT", zap.String("path", storagePath))
			
			// Add datastore option
			// Note: In a real implementation, you would use a proper datastore like leveldb
			// This is just a placeholder - actual implementation would depend on available datastores
			dhtOpts = append(dhtOpts, dht.Datastore(nil)) // Replace with actual datastore
		}
	}
	
	// Create the DHT
	kadDHT, err := dht.New(d.ctx, d.host, dhtOpts...)
	if err != nil {
		return fmt.Errorf("failed to create DHT: %w", err)
	}
	d.dht = kadDHT
	
	// Create the rendezvous discovery
	d.rendezvous = NewRendezvousDHT(d.ctx, d.host, d.dht, "clipman-peers", d.logger)
	
	// Connect to bootstrap peers first
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
	
	// Create and start the discovery ticker
	d.discoveryTicker = time.NewTicker(5 * time.Minute)
	
	// Start a goroutine for periodic discovery
	go d.discoverPeersLoop()
	
	// Do initial discovery immediately in a separate goroutine
	go d.runDiscovery()
	
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
	
	// Stop the discovery ticker if it exists
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
	d.logger.Info("Stopped DHT discovery")
	return nil
}

// Name returns the name of the discovery service
func (d *DHTDiscovery) Name() string {
	return "dht"
}

// discoverPeersLoop is the updated discovery routine that uses the RendezvousDHT
func (d *DHTDiscovery) discoverPeersLoop() {
	for {
		select {
		case <-d.ctx.Done():
			// Context cancelled, exit the routine
			return
		case <-d.discoveryTicker.C:
			// Time to do periodic discovery
			d.runDiscovery()
		}
	}
}

// runDiscovery performs the discovery process using both basic DHT and rendezvous
func (d *DHTDiscovery) runDiscovery() {
	d.mutex.RLock()
	if !d.started || d.dht == nil || d.rendezvous == nil {
		d.mutex.RUnlock()
		return
	}
	d.mutex.RUnlock()
	
	// First, advertise our presence using the rendezvous
	err := d.rendezvous.Advertise()
	if err != nil {
		d.logger.Warn("Failed to advertise on rendezvous", zap.Error(err))
	}
	
	// Then, discover peers using the rendezvous
	peers, err := d.rendezvous.Discover()
	if err != nil {
		d.logger.Warn("Failed to discover peers on rendezvous", zap.Error(err))
	} else {
		for _, pi := range peers {
			d.handleDiscoveredPeer(pi)
		}
	}
	
	// Also do basic DHT discovery as a fallback
	d.findDHTPeers()
}

// findDHTPeers searches for peers on the DHT using the provide/find mechanism
func (d *DHTDiscovery) findDHTPeers() {
	// Use a consistent rendezvous string
	rendezvous := "/clipman/peers/1.0.0"
	
	// Create a timeout context for this operation
	ctx, cancel := context.WithTimeout(d.ctx, 1*time.Minute)
	defer cancel()
	
	d.logger.Debug("Finding peers via basic DHT")
	
	// First, provide our peer info
	err := d.dht.Provide(ctx, convertToProvideKey(rendezvous), true)
	if err != nil {
		d.logger.Warn("Failed to provide record on DHT", zap.Error(err))
	}
	
	// Then, find providers
	providersChan := d.dht.FindProvidersAsync(ctx, convertToProvideKey(rendezvous), 100)
	
	// Process found providers
	for provider := range providersChan {
		// Skip if this is our own peer ID
		if provider.ID == d.host.ID() {
			continue
		}
		
		d.handleDiscoveredPeer(provider)
	}
}

// handleDiscoveredPeer processes a discovered peer from any source
func (d *DHTDiscovery) handleDiscoveredPeer(peerInfo peer.AddrInfo) {
	d.logger.Debug("Found peer via DHT", zap.String("peer_id", peerInfo.ID.String()))
	
	// Skip if no addresses are available
	if len(peerInfo.Addrs) == 0 {
		d.logger.Debug("Skipping peer with no addresses", zap.String("peer_id", peerInfo.ID.String()))
		return
	}
	
	// Add the peer to our host's peerstore with a longer TTL for discovered peers
	d.host.Peerstore().AddAddrs(peerInfo.ID, peerInfo.Addrs, time.Hour*24)
	
	// Create internal peer info
	internalInfo := InternalPeerInfo{
		ID:           peerInfo.ID.String(),
		LastSeen:     time.Now(),
		Addrs:        make([]string, 0, len(peerInfo.Addrs)),
		Capabilities: make(map[string]string),
		DeviceType:   "unknown", // Will be determined through protocol
	}
	
	// Convert addresses to strings
	for _, addr := range peerInfo.Addrs {
		internalInfo.Addrs = append(internalInfo.Addrs, addr.String())
	}
	
	// Notify the discovery manager
	d.manager.HandlePeerDiscovered(internalInfo)
}

// RendezvousDHT extends the DHT implementation with a more targeted discovery mechanism
// using a rendezvous point string that is application-specific
type RendezvousDHT struct {
	dht       *dht.IpfsDHT
	host      host.Host
	ctx       context.Context
	logger    *zap.Logger
	namespace string
}

// NewRendezvousDHT creates a new rendezvous-based discovery using the DHT
func NewRendezvousDHT(ctx context.Context, h host.Host, dht *dht.IpfsDHT, namespace string, logger *zap.Logger) *RendezvousDHT {
	return &RendezvousDHT{
		dht:       dht,
		host:      h,
		ctx:       ctx,
		logger:    logger.With(zap.String("component", "rendezvous-dht")),
		namespace: namespace,
	}
}

// Advertise announces our presence on the rendezvous point
func (r *RendezvousDHT) Advertise() error {
	r.logger.Debug("Advertising using rendezvous", zap.String("namespace", r.namespace))
	
	// Create a record with our info
	record := r.createRecord()
	
	// Get the rendezvous key
	key := r.getRendezvousKey()
	
	// Store the record in the DHT
	err := r.dht.PutValue(r.ctx, key, record)
	if err != nil {
		return fmt.Errorf("failed to put rendezvous record: %w", err)
	}
	
	return nil
}

// Discover looks for peers advertising on the rendezvous point
func (r *RendezvousDHT) Discover() ([]peer.AddrInfo, error) {
	r.logger.Debug("Discovering peers using rendezvous", zap.String("namespace", r.namespace))
	
	// Get the rendezvous key
	key := r.getRendezvousKey()
	
	// Look for peers with records on the DHT
	records, err := r.dht.GetValues(r.ctx, key, 100)
	if err != nil {
		return nil, fmt.Errorf("failed to get rendezvous records: %w", err)
	}
	
	// Process the records to extract peer info
	peers := make([]peer.AddrInfo, 0, len(records))
	for _, rec := range records {
		peer, err := r.peerFromRecord(rec)
		if err != nil {
			r.logger.Warn("Failed to parse peer from record", zap.Error(err))
			continue
		}
		
		// Skip our own peer
		if peer.ID == r.host.ID() {
			continue
		}
		
		peers = append(peers, peer)
	}
	
	return peers, nil
}

// createRecord creates a record of our peer information to store in the DHT
func (r *RendezvousDHT) createRecord() []byte {
	// In a real implementation, you'd likely use a more structured format,
	// perhaps using protocol buffers or msgpack. For simplicity, we're
	// just using a JSON encoding.
	
	// Get host addresses
	addrs := make([]string, 0, len(r.host.Addrs()))
	for _, addr := range r.host.Addrs() {
		addrs = append(addrs, addr.String())
	}
	
	// Create a record with our peer ID and addresses
	data := map[string]interface{}{
		"peer_id":  r.host.ID().String(),
		"addrs":    addrs,
		"app":      "clipman",
		"version":  "1.0.0", // This should come from a version constant
		"time":     time.Now().Unix(),
	}
	
	// Encode to JSON
	encoded, err := json.Marshal(data)
	if err != nil {
		r.logger.Error("Failed to encode peer record", zap.Error(err))
		// Return a minimal record as fallback
		return []byte(r.host.ID().String())
	}
	
	return encoded
}

// peerFromRecord extracts peer information from a record
func (r *RendezvousDHT) peerFromRecord(record []byte) (peer.AddrInfo, error) {
	// Decode the record
	var data map[string]interface{}
	if err := json.Unmarshal(record, &data); err != nil {
		// Try to interpret as a plain peer ID string as fallback
		if id, err := peer.Decode(string(record)); err == nil {
			return peer.AddrInfo{ID: id}, nil
		}
		return peer.AddrInfo{}, fmt.Errorf("failed to decode record: %w", err)
	}
	
	// Extract the peer ID
	peerIDStr, ok := data["peer_id"].(string)
	if !ok {
		return peer.AddrInfo{}, fmt.Errorf("record missing peer_id field")
	}
	
	// Parse the peer ID
	peerID, err := peer.Decode(peerIDStr)
	if err != nil {
		return peer.AddrInfo{}, fmt.Errorf("invalid peer ID in record: %w", err)
	}
	
	// Extract and parse addresses
	var addrs []peer.Multiaddr
	if addrList, ok := data["addrs"].([]interface{}); ok {
		for _, addrItem := range addrList {
			if addrStr, ok := addrItem.(string); ok {
				addr, err := multiaddr.NewMultiaddr(addrStr)
				if err != nil {
					r.logger.Warn("Invalid address in record", zap.String("addr", addrStr), zap.Error(err))
					continue
				}
				addrs = append(addrs, addr)
			}
		}
	}
	
	return peer.AddrInfo{
		ID:    peerID,
		Addrs: addrs,
	}, nil
}

// getRendezvousKey computes a key suitable for the DHT from the namespace
func (r *RendezvousDHT) getRendezvousKey() string {
	return fmt.Sprintf("/clipman/rendezvous/%s", r.namespace)
}

// ManualDiscovery allows users to manually add and remove peers
type ManualDiscovery struct {
	manager   *DiscoveryManager
	host      host.Host
	logger    *zap.Logger
	started   bool
	mutex     sync.RWMutex
}

// NewManualDiscovery creates a new manual discovery service
func NewManualDiscovery(manager *DiscoveryManager, host host.Host) *ManualDiscovery {
	return &ManualDiscovery{
		manager:  manager,
		host:     host,
		logger:   manager.logger.With(zap.String("discovery", "manual")),
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

// AddPeer manually adds a peer by multiaddress string
func (m *ManualDiscovery) AddPeer(addrStr string) error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	if !m.started {
		return fmt.Errorf("manual discovery service not started")
	}
	
	// Parse the multiaddress
	addrInfo, err := peer.AddrInfoFromString(addrStr)
	if err != nil {
		return fmt.Errorf("invalid multiaddress: %w", err)
	}
	
	// Skip if this is our own peer ID
	if addrInfo.ID == m.host.ID() {
		return fmt.Errorf("cannot add self as peer")
	}
	
	// Log the action
	m.logger.Info("Manually adding peer", zap.String("peer_id", addrInfo.ID.String()), zap.String("addr", addrStr))
	
	// Add the peer to the peerstore with a long TTL
	m.host.Peerstore().AddAddrs(addrInfo.ID, addrInfo.Addrs, time.Hour*24*7) // 1 week
	
	// Create peer info
	peerInfo := InternalPeerInfo{
		ID:           addrInfo.ID.String(),
		LastSeen:     time.Now(),
		Addrs:        make([]string, 0, len(addrInfo.Addrs)),
		Capabilities: make(map[string]string),
		DeviceType:   "unknown", // Will be determined through protocol
	}
	
	// Convert addresses to strings
	for _, addr := range addrInfo.Addrs {
		peerInfo.Addrs = append(peerInfo.Addrs, addr.String())
	}
	
	// Notify the discovery manager
	m.manager.HandlePeerDiscovered(peerInfo)
	
	return nil
}

// RemovePeer manually removes a peer
func (m *ManualDiscovery) RemovePeer(peerID string) error {
	m.mutex.RLock()
	defer m.mutex.RUnlock()
	
	if !m.started {
		return fmt.Errorf("manual discovery service not started")
	}
	
	// Parse the peer ID
	id, err := peer.Decode(peerID)
	if err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}
	
	// Log the action
	m.logger.Info("Manually removing peer", zap.String("peer_id", peerID))
	
	// Remove the peer from the peerstore
	m.host.Peerstore().ClearAddrs(id)
	
	return nil
}

// convertToProvideKey converts a string to a CID for DHT providing/finding
func convertToProvideKey(s string) cid.Cid {
	// Create a multihash from the string
	h, err := multihash.Sum([]byte(s), multihash.SHA2_256, -1)
	if err != nil {
		// This should never happen with SHA2_256
		panic(fmt.Sprintf("failed to create multihash: %v", err))
	}
	
	// Create a CID from the multihash
	return cid.NewCidV1(cid.Raw, h)
}

// InitializeDiscovery sets up peer discovery for the sync package
func InitializeDiscovery(ctx context.Context, host host.Host, config *SyncConfig, logger *zap.Logger) (*discovery.Manager, error) {
	// Map our SyncConfig to the discovery.Config
	discConfig := &discovery.Config{
		EnableMDNS:           config.DiscoveryMethod == "mdns" || config.DiscoveryMethod == "",
		EnableDHT:            config.DiscoveryMethod == "dht",
		BootstrapPeers:       config.DHTBootstrapPeers,
		PersistPeers:         config.PersistDiscoveredPeers,
		PeersPath:            config.DiscoveredPeersPath,
		MaxStoredPeers:       config.MaxStoredPeers,
		AutoReconnect:        config.AutoReconnectToPeers,
		DHTServerMode:        config.DHTServerMode,
		DHTPersistentStorage: config.DHTPersistentStorage,
		DHTStoragePath:       config.DHTStoragePath,
	}

	// Create the discovery manager
	manager := discovery.NewManager(ctx, host, discConfig, logger)

	return manager, nil
}

// SetupDiscoveryCallback sets up the callback for peer discovery
func SetupDiscoveryCallback(discoveryManager *discovery.Manager, handler func(peerInfo InternalPeerInfo)) {
	if discoveryManager == nil {
		return
	}

	// Create a callback that converts types.DiscoveryPeerInfo to our InternalPeerInfo
	callback := func(peerInfo types.DiscoveryPeerInfo) {
		if handler != nil {
			// Convert from DiscoveryPeerInfo to InternalPeerInfo
			internalPeerInfo := InternalPeerInfo{
				ID:           peerInfo.ID,
				Name:         peerInfo.Name,
				Addrs:        peerInfo.Addrs,
				Groups:       peerInfo.Groups,
				LastSeen:     peerInfo.LastSeen,
				Capabilities: peerInfo.Capabilities,
				Version:      peerInfo.Version,
				DeviceType:   peerInfo.DeviceType,
			}
			handler(internalPeerInfo)
		}
	}

	discoveryManager.SetPeerDiscoveredCallback(callback)
}

// StartDiscovery starts the discovery manager
func StartDiscovery(discoveryManager *discovery.Manager) error {
	if discoveryManager == nil {
		return fmt.Errorf("discovery manager is nil")
	}

	return discoveryManager.Start()
}

// StopDiscovery stops the discovery manager
func StopDiscovery(discoveryManager *discovery.Manager) error {
	if discoveryManager == nil {
		return nil
	}

	return discoveryManager.Stop()
}

// AddPeerByAddress adds a peer by its multiaddress string
func AddPeerByAddress(discoveryManager *discovery.Manager, address string) error {
	if discoveryManager == nil {
		return fmt.Errorf("discovery manager is nil")
	}

	manualDiscovery, err := discoveryManager.GetManualDiscovery()
	if err != nil {
		return fmt.Errorf("failed to get manual discovery: %w", err)
	}

	return manualDiscovery.AddPeer(address)
}

// RemovePeerByID removes a peer by its ID
func RemovePeerByID(discoveryManager *discovery.Manager, peerID string) error {
	if discoveryManager == nil {
		return fmt.Errorf("discovery manager is nil")
	}

	manualDiscovery, err := discoveryManager.GetManualDiscovery()
	if err != nil {
		return fmt.Errorf("failed to get manual discovery: %w", err)
	}

	return manualDiscovery.RemovePeer(peerID)
}

// SaveDiscoveredPeers saves the list of discovered peers
func SaveDiscoveredPeers(discoveryManager *discovery.Manager, peers map[string]InternalPeerInfo) error {
	if discoveryManager == nil {
		return fmt.Errorf("discovery manager is nil")
	}
	
	// Convert from our internal type to the types package
	discoveryPeers := make(map[string]types.DiscoveryPeerInfo)
	for id, peer := range peers {
		discoveryPeers[id] = types.DiscoveryPeerInfo{
			ID:           peer.ID,
			Name:         peer.Name,
			Addrs:        peer.Addrs,
			Groups:       peer.Groups,
			LastSeen:     peer.LastSeen,
			Capabilities: peer.Capabilities,
			Version:      peer.Version,
			DeviceType:   peer.DeviceType,
		}
	}

	return discoveryManager.SaveDiscoveredPeers(discoveryPeers)
}

// LoadDiscoveredPeers loads the list of discovered peers
func LoadDiscoveredPeers(discoveryManager *discovery.Manager) (map[string]InternalPeerInfo, error) {
	if discoveryManager == nil {
		return nil, fmt.Errorf("discovery manager is nil")
	}
	
	discoveryPeers, err := discoveryManager.LoadDiscoveredPeers()
	if err != nil {
		return nil, err
	}
	
	// Convert from types package to our internal type
	internalPeers := make(map[string]InternalPeerInfo)
	for id, peer := range discoveryPeers {
		internalPeers[id] = InternalPeerInfo{
			ID:           peer.ID,
			Name:         peer.Name,
			Addrs:        peer.Addrs,
			Groups:       peer.Groups,
			LastSeen:     peer.LastSeen,
			Capabilities: peer.Capabilities,
			Version:      peer.Version,
			DeviceType:   peer.DeviceType,
		}
	}
	
	return internalPeers, nil
} 