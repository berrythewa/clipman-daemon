// Package sync provides clipboard synchronization using libp2p
package sync

import (
	"context"
	"crypto/rand"
	"fmt"
	"sort"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/host/eventbus"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"go.uber.org/zap"
	"github.com/multiformats/go-multiaddr"
	"github.com/libp2p/go-libp2p/core/network"
)

// Node manages the libp2p host and core networking functionality
type Node struct {
	// Core components
	host          host.Host
	ctx           context.Context
	cancel        context.CancelFunc
	config        *SyncConfig
	logger        *zap.Logger
	
	// Discovery components
	discovery    *DiscoveryManager
	
	// PubSub for group communication
	pubsub        *pubsub.PubSub
	topics        map[string]*pubsub.Topic
	subscriptions map[string]*pubsub.Subscription
	
	// State
	started       bool
	peerStore     map[peer.ID]InternalPeerInfo
}

// NodeOption is a functional option for configuring the Node
type NodeOption func(*Node) error

// NewNode creates a new libp2p node using the provided configuration
func NewNode(ctx context.Context, cfg *config.Config, logger *zap.Logger, opts ...NodeOption) (*Node, error) {
	// Create cancellable context
	nodeCtx, cancel := context.WithCancel(ctx)
	
	// Load sync configuration from global config
	syncCfg := LoadSyncConfig(cfg)
	
	// Validate the configuration
	if err := ValidateSyncConfig(syncCfg); err != nil {
		cancel()
		return nil, fmt.Errorf("invalid sync configuration: %w", err)
	}
	
	// Get node-specific configuration
	nodeCfg := GetNodeConfig(syncCfg)
	
	// Initialize logger
	nodeLogger := logger
	if nodeLogger == nil {
		nodeLogger = zap.NewNop()
	}
	nodeLogger = nodeLogger.With(zap.String("component", "libp2p-node"))
	
	// Create a new host with options from config
	var privKey crypto.PrivKey
	var err error
	
	// Set up peer identity
	if nodeCfg.PeerIdentity != "" {
		// Try to load existing identity
		privKey, err = loadIdentityFromString(nodeCfg.PeerIdentity)
		if err != nil {
			nodeLogger.Warn("Failed to load peer identity, generating new one", zap.Error(err))
		}
	}
	
	// Generate new identity if needed
	if privKey == nil {
		privKey, _, err = crypto.GenerateKeyPairWithReader(crypto.Ed25519, -1, rand.Reader)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to generate peer identity: %w", err)
		}
		nodeLogger.Info("Generated new peer identity")
	}
	
	// Map config to libp2p options
	libp2pOpts, err := MapToLibp2pOptions(syncCfg, nodeCfg, nodeLogger)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create libp2p options: %w", err)
	}
	
	// Create the host
	h, err := libp2p.New(libp2pOpts...)
	if err != nil {
		cancel()
		return nil, fmt.Errorf("failed to create libp2p host: %w", err)
	}
	
	// Log host information
	nodeLogger.Info("Created libp2p host",
		zap.String("peer_id", h.ID().String()),
		zap.Strings("addresses", formatHostAddresses(h)),
	)
	
	// Create the node
	node := &Node{
		host:          h,
		ctx:           nodeCtx,
		cancel:        cancel,
		config:        syncCfg,
		logger:        nodeLogger,
		topics:        make(map[string]*pubsub.Topic),
		subscriptions: make(map[string]*pubsub.Subscription),
		peerStore:     make(map[peer.ID]InternalPeerInfo),
		started:       false,
	}
	
	// Create discovery manager
	node.discovery = NewDiscoveryManager(nodeCtx, h, syncCfg, nodeLogger)
	
	// Set callback for peer discovery
	node.discovery.SetPeerDiscoveredCallback(func(peerInfo InternalPeerInfo) {
		// Add peer to our store
		id, err := peer.Decode(peerInfo.ID)
		if err != nil {
			node.logger.Warn("Invalid peer ID from discovery", zap.String("peer_id", peerInfo.ID), zap.Error(err))
			return
		}
		
		// Add to peer store
		node.peerStore[id] = peerInfo
		
		// Attempt to connect to the peer
		ctx, cancel := context.WithTimeout(node.ctx, time.Second*10)
		defer cancel()
		
		// Create AddrInfo from the discovered peer
		addrInfo := peer.AddrInfo{
			ID: id,
			// Convert string addresses back to multiaddrs
			Addrs: make([]peer.Multiaddr, 0, len(peerInfo.Addrs)),
		}
		
		for _, addrStr := range peerInfo.Addrs {
			addr, err := peer.AddrInfoFromString(addrStr)
			if err != nil {
				node.logger.Warn("Invalid address for peer", 
					zap.String("peer_id", peerInfo.ID),
					zap.String("addr", addrStr),
					zap.Error(err))
				continue
			}
			addrInfo.Addrs = append(addrInfo.Addrs, addr.Addrs...)
		}
		
		// Connect to the peer
		if err := node.host.Connect(ctx, addrInfo); err != nil {
			node.logger.Warn("Failed to connect to discovered peer", 
				zap.String("peer_id", peerInfo.ID),
				zap.Error(err))
			return
		}
		
		node.logger.Info("Connected to discovered peer", zap.String("peer_id", peerInfo.ID))
	})
	
	// Apply any options
	for _, opt := range opts {
		if err := opt(node); err != nil {
			node.Close()
			return nil, err
		}
	}
	
	return node, nil
}

// Start starts the node and all its services
func (n *Node) Start() error {
	if n.started {
		return nil
	}
	
	// If configured to do so, load previously discovered peers
	if n.config.PersistDiscoveredPeers && n.config.AutoReconnectToPeers {
		// Load peers from storage
		previousPeers, err := n.discovery.LoadDiscoveredPeers()
		if err != nil {
			n.logger.Warn("Failed to load previously discovered peers", zap.Error(err))
		} else if len(previousPeers) > 0 {
			n.logger.Info("Loaded previously discovered peers", zap.Int("count", len(previousPeers)))
			
			// Add them to our peerstore
			for id, peerInfo := range previousPeers {
				// Convert string ID to peer.ID
				pid, err := peer.Decode(id)
				if err != nil {
					n.logger.Warn("Invalid peer ID in stored peers", zap.String("peer_id", id), zap.Error(err))
					continue
				}
				
				// Add to internal peer store
				n.peerStore[pid] = peerInfo
				
				// Try to connect in the background
				go func(pi InternalPeerInfo) {
					// Create a timeout context for the connection
					ctx, cancel := context.WithTimeout(n.ctx, 10*time.Second)
					defer cancel()
					
					addrInfo, err := peerInfoToAddrInfo(pi)
					if err != nil {
						n.logger.Warn("Failed to parse addresses for stored peer", 
							zap.String("peer_id", pi.ID), 
							zap.Error(err))
						return
					}
					
					if err := n.host.Connect(ctx, addrInfo); err != nil {
						n.logger.Debug("Could not connect to stored peer, will try again later", 
							zap.String("peer_id", pi.ID), 
							zap.Error(err))
					} else {
						n.logger.Info("Connected to stored peer", zap.String("peer_id", pi.ID))
					}
				}(peerInfo)
			}
		}
	}
	
	// Start discovery services
	if err := n.discovery.Start(); err != nil {
		return fmt.Errorf("failed to start discovery services: %w", err)
	}
	
	// Initialize pubsub if needed
	if err := n.setupPubSub(); err != nil {
		return fmt.Errorf("failed to set up pubsub: %w", err)
	}
	
	// Start a goroutine to periodically save discovered peers
	if n.config.PersistDiscoveredPeers {
		go n.savePeersRoutine()
	}
	
	n.started = true
	n.logger.Info("Node started", zap.String("peer_id", n.host.ID().String()))
	return nil
}

// Stop stops the node and all its services
func (n *Node) Stop() error {
	if !n.started {
		return nil
	}
	
	// Stop discovery services
	if err := n.discovery.Stop(); err != nil {
		n.logger.Warn("Error stopping discovery services", zap.Error(err))
	}
	
	// Unsubscribe from all topics
	for name, sub := range n.subscriptions {
		sub.Cancel()
		delete(n.subscriptions, name)
	}
	
	// Close all topics
	for name := range n.topics {
		delete(n.topics, name)
	}
	
	// Cancel context to signal shutdown to any goroutines
	n.cancel()
	
	// Close the host
	if err := n.host.Close(); err != nil {
		return fmt.Errorf("failed to close host: %w", err)
	}
	
	n.started = false
	n.logger.Info("Node stopped")
	return nil
}

// Host returns the libp2p host
func (n *Node) Host() host.Host {
	return n.host
}

// ID returns the node's peer ID
func (n *Node) ID() peer.ID {
	return n.host.ID()
}

// AddPeer adds a peer to the peerstore
func (n *Node) AddPeer(pi InternalPeerInfo) {
	id, err := peer.Decode(pi.ID)
	if err != nil {
		n.logger.Warn("Failed to decode peer ID", zap.String("peer_id", pi.ID), zap.Error(err))
		return
	}
	n.peerStore[id] = pi
}

// RemovePeer removes a peer from the peerstore
func (n *Node) RemovePeer(id peer.ID) {
	delete(n.peerStore, id)
}

// GetPeers returns all known peers
func (n *Node) GetPeers() []InternalPeerInfo {
	peers := make([]InternalPeerInfo, 0, len(n.peerStore))
	for _, p := range n.peerStore {
		peers = append(peers, p)
	}
	return peers
}

// AddPeerByAddress adds a peer by its multiaddress string
func (n *Node) AddPeerByAddress(addrStr string) error {
	// Get the manual discovery service
	manualDiscovery, err := n.discovery.GetManualDiscovery()
	if err != nil {
		return fmt.Errorf("failed to get manual discovery service: %w", err)
	}
	
	// Add the peer
	return manualDiscovery.AddPeer(addrStr)
}

// RemovePeerByID removes a peer by its ID
func (n *Node) RemovePeerByID(peerID string) error {
	// Get the manual discovery service
	manualDiscovery, err := n.discovery.GetManualDiscovery()
	if err != nil {
		return fmt.Errorf("failed to get manual discovery service: %w", err)
	}
	
	// Remove the peer
	return manualDiscovery.RemovePeer(peerID)
}

// DisconnectPeer disconnects from a peer but keeps it in the peerstore
func (n *Node) DisconnectPeer(peerID string) error {
	id, err := peer.Decode(peerID)
	if err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}
	
	// Check if connected
	if n.host.Network().Connectedness(id) != network.Connected {
		return fmt.Errorf("not connected to peer %s", peerID)
	}
	
	// Disconnect
	return n.host.Network().ClosePeer(id)
}

// GetConnectedPeers returns a list of currently connected peers
func (n *Node) GetConnectedPeers() []InternalPeerInfo {
	peers := make([]InternalPeerInfo, 0)
	
	// Get all connections
	for _, conn := range n.host.Network().Conns() {
		pid := conn.RemotePeer()
		
		// Look up in our peerstore
		if pi, ok := n.peerStore[pid]; ok {
			// Update last seen time
			pi.LastSeen = time.Now()
			peers = append(peers, pi)
		} else {
			// Create a basic entry for this peer
			addrs := make([]string, 0)
			for _, addr := range n.host.Peerstore().Addrs(pid) {
				addrs = append(addrs, addr.String())
			}
			
			peers = append(peers, InternalPeerInfo{
				ID:           pid.String(),
				LastSeen:     time.Now(),
				Addrs:        addrs,
				Capabilities: make(map[string]string),
				DeviceType:   "unknown",
			})
		}
	}
	
	return peers
}

// JoinTopic joins a pubsub topic for group communication
func (n *Node) JoinTopic(group string) (*pubsub.Topic, *pubsub.Subscription, error) {
	if n.pubsub == nil {
		return nil, nil, fmt.Errorf("pubsub not initialized")
	}
	
	// Check if we're already subscribed
	if topic, ok := n.topics[group]; ok {
		if sub, ok := n.subscriptions[group]; ok {
			return topic, sub, nil
		}
	}
	
	// Create or join the topic
	topic, err := n.pubsub.Join(topicName(group))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to join topic %s: %w", group, err)
	}
	
	// Subscribe to the topic
	sub, err := topic.Subscribe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to subscribe to topic %s: %w", group, err)
	}
	
	// Store the topic and subscription
	n.topics[group] = topic
	n.subscriptions[group] = sub
	
	n.logger.Info("Joined group", zap.String("group", group))
	return topic, sub, nil
}

// LeaveTopic leaves a pubsub topic
func (n *Node) LeaveTopic(group string) error {
	// Cancel subscription if it exists
	if sub, ok := n.subscriptions[group]; ok {
		sub.Cancel()
		delete(n.subscriptions, group)
	}
	
	// Remove topic
	if _, ok := n.topics[group]; ok {
		delete(n.topics, group)
	}
	
	n.logger.Info("Left group", zap.String("group", group))
	return nil
}

// PublishToTopic publishes a message to a topic
func (n *Node) PublishToTopic(group string, data []byte) error {
	topic, ok := n.topics[group]
	if !ok {
		return fmt.Errorf("not subscribed to group %s", group)
	}
	
	err := topic.Publish(n.ctx, data)
	if err != nil {
		return fmt.Errorf("failed to publish to topic %s: %w", group, err)
	}
	
	return nil
}

// GetConfig returns the node's configuration
func (n *Node) GetConfig() *SyncConfig {
	return n.config
}

// Close closes the node and releases resources
func (n *Node) Close() error {
	return n.Stop()
}

// Setup functions

// setupPubSub sets up the pubsub system for group messaging
func (n *Node) setupPubSub() error {
	// Create pubsub instance (using GossipSub)
	ps, err := pubsub.NewGossipSub(n.ctx, n.host)
	if err != nil {
		return fmt.Errorf("failed to create pubsub: %w", err)
	}
	
	n.pubsub = ps
	n.logger.Info("Initialized pubsub system")
	return nil
}

// Helper functions

// topicName returns the full topic name for a group
func topicName(group string) string {
	return fmt.Sprintf("clipman/%s", group)
}

// formatHostAddresses returns the host's addresses as strings
func formatHostAddresses(h host.Host) []string {
	hostAddr := make([]string, 0, len(h.Addrs()))
	for _, addr := range h.Addrs() {
		hostAddr = append(hostAddr, fmt.Sprintf("%s/p2p/%s", addr.String(), h.ID().String()))
	}
	return hostAddr
}

// savePeersRoutine periodically saves discovered peers to disk
func (n *Node) savePeersRoutine() {
	// Save peers every 5 minutes
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-n.ctx.Done():
			// Context canceled, save peers one last time before exiting
			n.savePeers()
			return
		case <-ticker.C:
			// Time to save peers
			n.savePeers()
		}
	}
}

// savePeers saves the current set of peers to disk
func (n *Node) savePeers() {
	n.logger.Debug("Saving discovered peers to disk")
	
	// Create a map of string ID to peer info for the discovery manager
	peerMap := make(map[string]InternalPeerInfo)
	
	// Convert our peer store to the format expected by the discovery manager
	for id, peerInfo := range n.peerStore {
		// Only save if not too old (within the last 30 days)
		cutoff := time.Now().Add(-30 * 24 * time.Hour)
		if peerInfo.LastSeen.After(cutoff) {
			peerMap[id.String()] = peerInfo
		}
	}
	
	// Limit the number of stored peers if needed
	if len(peerMap) > n.config.MaxStoredPeers && n.config.MaxStoredPeers > 0 {
		// Create a slice of peers sorted by last seen time
		peers := make([]InternalPeerInfo, 0, len(peerMap))
		for _, pi := range peerMap {
			peers = append(peers, pi)
		}
		
		// Sort by LastSeen, most recent first
		sort.Slice(peers, func(i, j int) bool {
			return peers[i].LastSeen.After(peers[j].LastSeen)
		})
		
		// Create a new map with only the most recently seen peers
		newMap := make(map[string]InternalPeerInfo)
		for i := 0; i < n.config.MaxStoredPeers && i < len(peers); i++ {
			newMap[peers[i].ID] = peers[i]
		}
		
		peerMap = newMap
	}
	
	// Save the peers
	if err := n.discovery.SaveDiscoveredPeers(peerMap); err != nil {
		n.logger.Warn("Failed to save discovered peers", zap.Error(err))
	}
}

// peerInfoToAddrInfo converts an InternalPeerInfo to a peer.AddrInfo
func peerInfoToAddrInfo(pi InternalPeerInfo) (peer.AddrInfo, error) {
	id, err := peer.Decode(pi.ID)
	if err != nil {
		return peer.AddrInfo{}, fmt.Errorf("invalid peer ID: %w", err)
	}
	
	addrs := make([]peer.Multiaddr, 0, len(pi.Addrs))
	for _, addrStr := range pi.Addrs {
		addr, err := multiaddr.NewMultiaddr(addrStr)
		if err != nil {
			continue // Skip invalid addresses
		}
		addrs = append(addrs, addr)
	}
	
	return peer.AddrInfo{
		ID:    id,
		Addrs: addrs,
	}, nil
}