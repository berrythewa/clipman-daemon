// Package mdns provides mDNS-based peer discovery for local networks
package mdns

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/sync"
	"github.com/berrythewa/clipman-daemon/internal/sync/discovery"
	"github.com/grandcat/zeroconf"
	"go.uber.org/zap"
)

const (
	serviceType = "_clipman._tcp"
	domain      = "local."
	ttl         = 120
)

// MDNSDiscovery provides peer discovery via mDNS
type MDNSDiscovery struct {
	*discovery.BaseDiscovery
	server        *zeroconf.Server
	browser       *zeroconf.Resolver
	ctx           context.Context
	cancel        context.CancelFunc
	deviceID      string
	deviceName    string
	announceTimer *time.Ticker
	peerTimeout   time.Duration
	mutex         sync.Mutex
	running       bool
}

// MDNSOptions contains options specific to mDNS discovery
type MDNSOptions struct {
	*discovery.DiscoveryOptions
	Port           int    // Port to announce services on
	Interface      string // Specific network interface to use (empty for all)
	Domain         string // Domain for mDNS (default: local.)
	BrowseInterval int    // How often to browse for services (in seconds)
}

// DefaultMDNSOptions returns default options for mDNS discovery
func DefaultMDNSOptions() *MDNSOptions {
	return &MDNSOptions{
		DiscoveryOptions: &discovery.DiscoveryOptions{
			AnnounceInterval: 60,
			PeerTimeout:      120,
			DeviceID:         "",
			DeviceName:       "Clipman Device",
			Logger:           nil,
		},
		Port:           5353,
		Interface:      "",
		Domain:         domain,
		BrowseInterval: 30,
	}
}

// NewMDNSDiscovery creates a new mDNS discovery
func NewMDNSDiscovery(options *MDNSOptions) (*MDNSDiscovery, error) {
	if options == nil {
		options = DefaultMDNSOptions()
	}
	
	// Use the base discovery implementation
	base := discovery.NewBaseDiscovery(options.DiscoveryOptions)
	
	ctx, cancel := context.WithCancel(context.Background())
	
	// Create the mDNS discovery
	d := &MDNSDiscovery{
		BaseDiscovery: base,
		deviceID:      options.DeviceID,
		deviceName:    options.DeviceName,
		ctx:           ctx,
		cancel:        cancel,
		peerTimeout:   time.Duration(options.PeerTimeout) * time.Second,
		running:       false,
	}
	
	return d, nil
}

// Start starts the mDNS discovery
func (d *MDNSDiscovery) Start() error {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	if d.running {
		return nil
	}
	
	// Start announcing our presence
	err := d.startServer()
	if err != nil {
		return fmt.Errorf("failed to start mDNS server: %w", err)
	}
	
	// Start discovering peers
	err = d.startBrowsing()
	if err != nil {
		d.stopServer()
		return fmt.Errorf("failed to start mDNS browsing: %w", err)
	}
	
	// Start the announce timer
	d.announceTimer = time.NewTicker(time.Duration(d.options.AnnounceInterval) * time.Second)
	go func() {
		for {
			select {
			case <-d.announceTimer.C:
				// Refresh our presence
				d.refreshAnnouncement()
			case <-d.ctx.Done():
				return
			}
		}
	}()
	
	d.running = true
	d.logger.Info("mDNS discovery started",
		zap.String("device_id", d.deviceID),
		zap.String("device_name", d.deviceName))
	
	return nil
}

// Stop stops the mDNS discovery
func (d *MDNSDiscovery) Stop() error {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	if !d.running {
		return nil
	}
	
	// Stop the announce timer
	if d.announceTimer != nil {
		d.announceTimer.Stop()
	}
	
	// Stop the browser
	d.stopBrowsing()
	
	// Stop the server
	d.stopServer()
	
	// Cancel the context
	d.cancel()
	
	d.running = false
	d.logger.Info("mDNS discovery stopped")
	
	return nil
}

// Announce announces our presence on the network
func (d *MDNSDiscovery) Announce(info sync.PeerInfo) error {
	d.mutex.Lock()
	defer d.mutex.Unlock()
	
	// Update our device information
	d.deviceID = info.ID
	d.deviceName = info.Name
	
	// If we're running, refresh our announcement
	if d.running {
		return d.refreshAnnouncement()
	}
	
	return nil
}

// SetPeerDiscoveryHandler sets the handler for peer discovery events
func (d *MDNSDiscovery) SetPeerDiscoveryHandler(handler func(sync.PeerInfo)) {
	d.BaseDiscovery.SetPeerDiscoveryHandler(func(peer sync.PeerInfo, discovered bool) {
		if discovered && handler != nil {
			handler(peer)
		}
	})
}

// EnableMDNS enables or disables mDNS discovery
func (d *MDNSDiscovery) EnableMDNS(enabled bool) error {
	if enabled {
		if !d.running {
			return d.Start()
		}
	} else {
		if d.running {
			return d.Stop()
		}
	}
	
	return nil
}

// EnableMQTTDiscovery is a no-op for mDNS discovery
func (d *MDNSDiscovery) EnableMQTTDiscovery(enabled bool) error {
	// MQTT discovery is not supported by mDNS
	return nil
}

// startServer starts the mDNS server
func (d *MDNSDiscovery) startServer() error {
	// Create a metadata map for the service
	meta := map[string]string{
		"id":       d.deviceID,
		"name":     d.deviceName,
		"version":  "1.0.0",
		"protocol": "clipman-sync",
		"type":     "desktop",
	}
	
	// Convert the metadata to text records
	var txtRecords []string
	for k, v := range meta {
		txtRecords = append(txtRecords, fmt.Sprintf("%s=%s", k, v))
	}
	
	// Create the server
	var err error
	d.server, err = zeroconf.Register(
		d.deviceName,
		serviceType,
		domain,
		5353, // Port
		txtRecords,
		nil, // Use all interfaces
	)
	
	if err != nil {
		return err
	}
	
	return nil
}

// stopServer stops the mDNS server
func (d *MDNSDiscovery) stopServer() {
	if d.server != nil {
		d.server.Shutdown()
		d.server = nil
	}
}

// startBrowsing starts browsing for peers
func (d *MDNSDiscovery) startBrowsing() error {
	resolver, err := zeroconf.NewResolver(nil)
	if err != nil {
		return err
	}
	
	d.browser = resolver
	
	// Create a channel to receive service entries
	entries := make(chan *zeroconf.ServiceEntry, 10)
	
	// Start browsing
	err = resolver.Browse(d.ctx, serviceType, domain, entries)
	if err != nil {
		return err
	}
	
	// Handle discovered services
	go func() {
		for {
			select {
			case entry := <-entries:
				// Process the service entry
				d.handleServiceEntry(entry)
			case <-d.ctx.Done():
				return
			}
		}
	}()
	
	return nil
}

// stopBrowsing stops browsing for peers
func (d *MDNSDiscovery) stopBrowsing() {
	// The resolver will stop when the context is cancelled
	d.browser = nil
}

// refreshAnnouncement refreshes our announcement
func (d *MDNSDiscovery) refreshAnnouncement() error {
	// Stop the server
	d.stopServer()
	
	// Start the server again
	return d.startServer()
}

// handleServiceEntry processes a discovered service entry
func (d *MDNSDiscovery) handleServiceEntry(entry *zeroconf.ServiceEntry) {
	// Extract metadata from TXT records
	meta := make(map[string]string)
	for _, txt := range entry.Text {
		// Parse key=value pairs
		// This is a simple implementation, could be improved
		for i := 0; i < len(txt); i++ {
			if txt[i] == '=' {
				meta[txt[:i]] = txt[i+1:]
				break
			}
		}
	}
	
	// Check if this is a clipman peer
	id, ok := meta["id"]
	if !ok {
		return
	}
	
	// Skip our own announcements
	if id == d.deviceID {
		return
	}
	
	// Get IP address (prefer IPv4)
	var addr string
	if len(entry.AddrIPv4) > 0 {
		addr = entry.AddrIPv4[0].String()
	} else if len(entry.AddrIPv6) > 0 {
		addr = entry.AddrIPv6[0].String()
	} else {
		return
	}
	
	// Create peer info
	peer := sync.PeerInfo{
		ID:           id,
		Name:         meta["name"],
		Address:      fmt.Sprintf("%s:%d", addr, entry.Port),
		Groups:       []string{},
		LastSeen:     time.Now(),
		Capabilities: meta,
		Version:      meta["version"],
		DeviceType:   meta["type"],
	}
	
	// Parse JSON groups if available
	if groups, ok := meta["groups"]; ok {
		var groupList []string
		if err := json.Unmarshal([]byte(groups), &groupList); err == nil {
			peer.Groups = groupList
		}
	}
	
	// Add or update the peer
	isNew := d.addOrUpdatePeer(peer)
	if isNew {
		d.notifyPeerDiscovered(peer)
	}
}

// Factory implementation for mDNS discovery

// Factory creates mDNS discovery instances
type Factory struct{}

// NewFactory creates a new mDNS discovery factory
func NewFactory() *Factory {
	return &Factory{}
}

// init registers the factory with the discovery registry
func init() {
	discovery.RegisterDiscoveryFactory(string(discovery.MethodMDNS), NewFactory())
}

// CreateDiscovery creates a discovery instance with the given options
func (f *Factory) CreateDiscovery(options interface{}) (sync.Discovery, error) {
	switch opts := options.(type) {
	case *MDNSOptions:
		return NewMDNSDiscovery(opts)
	case discovery.DiscoveryOptions:
		mdnsOpts := &MDNSOptions{
			DiscoveryOptions: &opts,
		}
		return NewMDNSDiscovery(mdnsOpts)
	case map[string]interface{}:
		// Convert from map to options
		mdnsOpts := DefaultMDNSOptions()
		
		// Apply options from map
		if v, ok := opts["announceInterval"].(int); ok {
			mdnsOpts.AnnounceInterval = v
		}
		if v, ok := opts["peerTimeout"].(int); ok {
			mdnsOpts.PeerTimeout = v
		}
		if v, ok := opts["deviceID"].(string); ok {
			mdnsOpts.DeviceID = v
		}
		if v, ok := opts["deviceName"].(string); ok {
			mdnsOpts.DeviceName = v
		}
		if v, ok := opts["port"].(int); ok {
			mdnsOpts.Port = v
		}
		if v, ok := opts["interface"].(string); ok {
			mdnsOpts.Interface = v
		}
		
		return NewMDNSDiscovery(mdnsOpts)
	default:
		return nil, fmt.Errorf("unsupported options type: %T", options)
	}
}

// SupportsConfig checks if this factory supports the given configuration
func (f *Factory) SupportsConfig(config interface{}) bool {
	// This factory supports all configurations for now
	return true
} 