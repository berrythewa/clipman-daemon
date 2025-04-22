// Package sync provides clipboard synchronization using libp2p
package sync

import (
	"context"
	"crypto/rand"
	// "encoding/json"
	"fmt"
	// "os"
	// "path/filepath"
	"sort"
	// "sync"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/sync/discovery"
	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	// "github.com/libp2p/go-libp2p/p2p/host/eventbus"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"go.uber.org/zap"
	"github.com/multiformats/go-multiaddr"
	// "github.com/libp2p/go-libp2p/core/network"
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
	discovery    *discovery.Manager
	
	// Protocol components
	protocols    *ProtocolManager
	pairing      *PairingManager
	
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
	
	// Create discovery manager with new discovery package
	discoveryConfig := ConvertToDiscoveryConfig(syncCfg)
	node.discovery = discovery.NewManager(nodeCtx, h, discoveryConfig, nodeLogger)
	
	// Create protocol manager
	node.protocols = NewProtocolManager(nodeCtx, h, syncCfg, nodeLogger)
	
	// Create pairing manager
	node.pairing = NewPairingManager(nodeCtx, h, node.protocols, syncCfg, nodeLogger)
	
	// Add pairing discovery service if configured to use paired discovery
	if syncCfg.DiscoveryMethod == "paired" {
		pairingDiscovery := NewPairingDiscoveryService(node.pairing, h)
		node.discovery.AddService("paired", pairingDiscovery)
	}
	
	// Set callback for peer discovery
	node.discovery.SetPeerDiscoveredCallback(func(peerInfo types.DiscoveryPeerInfo) {
		// Add peer to our store
		id, err := peer.Decode(peerInfo.ID)
		if err != nil {
			node.logger.Warn("Invalid peer ID from discovery", zap.String("peer_id", peerInfo.ID), zap.Error(err))
			return
		}
		
		// Convert discovery peer info to internal peer info
		internalPeerInfo := InternalPeerInfo{
			ID:          peerInfo.ID,
			Name:        peerInfo.Name,
			Addrs:       peerInfo.Addrs,
			DeviceType:  peerInfo.DeviceType,
			LastSeen:    peerInfo.LastSeen,
			Version:     peerInfo.Version,
			Groups:      peerInfo.Groups,
			Capabilities: peerInfo.Capabilities,
		}
		
		// Add to peer store
		node.peerStore[id] = internalPeerInfo
		
		// Attempt to connect to the peer
		ctx, cancel := context.WithTimeout(node.ctx, time.Second*10)
		defer cancel()
		
		// Create AddrInfo from the discovered peer
		addrInfo := peer.AddrInfo{
			ID: id,
			// Convert string addresses back to multiaddrs
			Addrs: make([]multiaddr.Multiaddr, 0, len(peerInfo.Addrs)),
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
		// Load peers from storage using the new discovery package
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
				
				// Convert to internal peer info
				internalPeerInfo := InternalPeerInfo{
					ID:           peerInfo.ID,
					Name:         peerInfo.Name,
					Addrs:        peerInfo.Addrs,
					DeviceType:   peerInfo.DeviceType,
					LastSeen:     peerInfo.LastSeen,
					Version:      peerInfo.Version,
					Groups:       peerInfo.Groups,
					Capabilities: peerInfo.Capabilities,
				}
				
				// Add to internal peer store
				n.peerStore[pid] = internalPeerInfo
				
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
				}(internalPeerInfo)
			}
		}
	}
	
	// Start discovery services
	n.logger.Info("Starting discovery services")
	if err := n.discovery.Start(); err != nil {
		return fmt.Errorf("failed to start discovery services: %w", err)
	}
	
	// Start protocol handlers
	n.logger.Info("Starting protocol handlers")
	if err := n.protocols.Start(); err != nil {
		return fmt.Errorf("failed to start protocol handlers: %w", err)
	}
	
	// Set up pubsub if the node supports it
	if err := n.setupPubSub(); err != nil {
		n.logger.Warn("Failed to set up pubsub", zap.Error(err))
	}
	
	// Set up periodic saving of discovered peers
	if n.config.PersistDiscoveredPeers {
		go n.savePeersRoutine()
	}
	
	n.started = true
	n.logger.Info("Node started successfully")
	return nil
}

// Stop stops the node and all its services
func (n *Node) Stop() error {
	if !n.started {
		return nil
	}
	
	// Save peers before shutting down if configured
	if n.config.PersistDiscoveredPeers {
		if err := n.savePeers(); err != nil {
			n.logger.Warn("Failed to save peers during shutdown", zap.Error(err))
		}
	}
	
	// Stop protocol handlers
	n.logger.Info("Stopping protocol handlers")
	if err := n.protocols.Stop(); err != nil {
		n.logger.Warn("Failed to stop protocol handlers", zap.Error(err))
	}
	
	// Stop discovery services
	n.logger.Info("Stopping discovery services")
	if err := n.discovery.Stop(); err != nil {
		n.logger.Warn("Failed to stop discovery services", zap.Error(err))
	}
	
	// Close all pubsub subscriptions
	for group, sub := range n.subscriptions {
		sub.Cancel()
		delete(n.subscriptions, group)
	}
	
	// Close all topics
	n.topics = make(map[string]*pubsub.Topic)
	
	n.started = false
	n.logger.Info("Node stopped successfully")
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

// Protocols returns the protocol manager
func (n *Node) Protocols() *ProtocolManager {
	return n.protocols
}

// Pairing returns the pairing manager
func (n *Node) Pairing() *PairingManager {
	return n.pairing
}

// GetManualDiscoveryService returns the manual discovery service
func (n *Node) GetManualDiscoveryService() (*discovery.ManualDiscovery, error) {
	return n.discovery.GetManualDiscovery()
}

// AddPeer adds a peer to the internal peer store
func (n *Node) AddPeer(pi InternalPeerInfo) {
	id, err := peer.Decode(pi.ID)
	if err != nil {
		n.logger.Warn("Invalid peer ID", zap.String("peer_id", pi.ID), zap.Error(err))
		return
	}
	n.peerStore[id] = pi
}

// RemovePeer removes a peer from the internal store
func (n *Node) RemovePeer(id peer.ID) {
	delete(n.peerStore, id)
}

// GetPeers returns all peers in the internal store
func (n *Node) GetPeers() []InternalPeerInfo {
	peers := make([]InternalPeerInfo, 0, len(n.peerStore))
	for _, peer := range n.peerStore {
		peers = append(peers, peer)
	}
	return peers
}

// AddPeerByAddress adds a peer by its multiaddress string
func (n *Node) AddPeerByAddress(addrStr string) error {
	manualDiscovery, err := n.GetManualDiscoveryService()
	if err != nil {
		return fmt.Errorf("failed to get manual discovery service: %w", err)
	}
	
	return manualDiscovery.AddPeer(addrStr)
}

// RemovePeerByID removes a peer by its ID string
func (n *Node) RemovePeerByID(peerID string) error {
	id, err := peer.Decode(peerID)
	if err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}
	
	// Remove from peerstore
	n.RemovePeer(id)
	
	// Remove from host's peerstore
	n.host.Peerstore().ClearAddrs(id)
	
	return nil
}

// DisconnectPeer disconnects from a peer but keeps it in the peerstore
func (n *Node) DisconnectPeer(peerID string) error {
	id, err := peer.Decode(peerID)
	if err != nil {
		return fmt.Errorf("invalid peer ID: %w", err)
	}
	
	// Disconnect all connections to this peer
	for _, conn := range n.host.Network().ConnsToPeer(id) {
		if err := conn.Close(); err != nil {
			n.logger.Warn("Failed to close connection to peer", 
				zap.String("peer_id", peerID), 
				zap.Error(err))
		}
	}
	
	return nil
}

// GetConnectedPeers returns a list of connected peers
func (n *Node) GetConnectedPeers() []InternalPeerInfo {
	// Get all connected peers from the network
	connectedPeers := n.host.Network().Peers()
	result := make([]InternalPeerInfo, 0, len(connectedPeers))
	
	for _, id := range connectedPeers {
		// Get from our internal store if available
		if info, exists := n.peerStore[id]; exists {
			// Update last seen time
			info.LastSeen = time.Now()
			// Add to result
			result = append(result, info)
		} else {
			// Create a basic peer info for connected peers not in our store
			info := InternalPeerInfo{
				ID:       id.String(),
				Name:     "Peer-" + id.ShortString(),
				LastSeen: time.Now(),
			}
			
			// Get addresses from peerstore
			addrs := n.host.Peerstore().Addrs(id)
			addrStrings := make([]string, 0, len(addrs))
			for _, addr := range addrs {
				addrStrings = append(addrStrings, addr.String())
			}
			info.Addrs = addrStrings
			
			// Add to result
			result = append(result, info)
		}
	}
	
	// Sort by last seen time (most recent first)
	sort.Slice(result, func(i, j int) bool {
		return result[i].LastSeen.After(result[j].LastSeen)
	})
	
	return result
}

// JoinTopic joins a pubsub topic for a group
func (n *Node) JoinTopic(group string) (*pubsub.Topic, *pubsub.Subscription, error) {
	if n.pubsub == nil {
		return nil, nil, fmt.Errorf("pubsub not available")
	}
	
	// Check if already joined
	if topic, exists := n.topics[group]; exists {
		if sub, exists := n.subscriptions[group]; exists {
			return topic, sub, nil
		}
	}
	
	// Join the topic
	topicName := topicName(group)
	topic, err := n.pubsub.Join(topicName)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to join topic %s: %w", topicName, err)
	}
	
	// Create subscription
	sub, err := topic.Subscribe()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to subscribe to topic %s: %w", topicName, err)
	}
	
	// Store topic and subscription
	n.topics[group] = topic
	n.subscriptions[group] = sub
	
	n.logger.Info("Joined group topic", 
		zap.String("group", group),
		zap.String("topic", topicName))
	
	return topic, sub, nil
}

// LeaveTopic leaves a pubsub topic for a group
func (n *Node) LeaveTopic(group string) error {
	// Check if joined
	sub, exists := n.subscriptions[group]
	if !exists {
		return nil
	}
	
	// Cancel subscription
	sub.Cancel()
	delete(n.subscriptions, group)
	
	// Close topic
	topic, exists := n.topics[group]
	if exists {
		if err := topic.Close(); err != nil {
			n.logger.Warn("Failed to close topic", 
				zap.String("group", group),
				zap.Error(err))
		}
		delete(n.topics, group)
	}
	
	n.logger.Info("Left group topic", zap.String("group", group))
	return nil
}

// PublishToTopic publishes data to a topic
func (n *Node) PublishToTopic(group string, data []byte) error {
	// Get the topic
	topic, exists := n.topics[group]
	if !exists {
		return fmt.Errorf("not subscribed to group %s", group)
	}
	
	// Publish the data
	err := topic.Publish(n.ctx, data)
	if err != nil {
		return fmt.Errorf("failed to publish to group %s: %w", group, err)
	}
	
	return nil
}

// GetConfig returns the node's configuration
func (n *Node) GetConfig() *SyncConfig {
	return n.config
}

// Close closes the node
func (n *Node) Close() error {
	if n.started {
		if err := n.Stop(); err != nil {
			return err
		}
	}
	n.cancel()
	return nil
}

// setupPubSub initializes the pubsub system
func (n *Node) setupPubSub() error {
	// Create pubsub
	ps, err := pubsub.NewGossipSub(n.ctx, n.host)
	if err != nil {
		return fmt.Errorf("failed to create pubsub: %w", err)
	}
	
	n.pubsub = ps
	n.logger.Info("PubSub initialized")
	return nil
}

// topicName converts a group name to a topic name
func topicName(group string) string {
	return "clipman-" + group
}

// formatHostAddresses formats the host's addresses for logging
func formatHostAddresses(h host.Host) []string {
	addrs := make([]string, 0, len(h.Addrs()))
	for _, addr := range h.Addrs() {
		addrs = append(addrs, addr.String()+"/p2p/"+h.ID().String())
	}
	return addrs
}

// savePeersRoutine periodically saves the discovered peers
func (n *Node) savePeersRoutine() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			if err := n.savePeers(); err != nil {
				n.logger.Warn("Failed to save peers", zap.Error(err))
			}
		case <-n.ctx.Done():
			return
		}
	}
}

// savePeers saves the discovered peers to disk using the discovery manager
func (n *Node) savePeers() error {
	// Skip if there are no peers
	if len(n.peerStore) == 0 {
		return nil
	}
	
	// Skip if not configured to persist peers
	if !n.config.PersistDiscoveredPeers {
		return nil
	}
	
	// Convert internal peers to discovery peer info
	discoveryPeers := make(map[string]types.DiscoveryPeerInfo, len(n.peerStore))
	
	for id, peerInfo := range n.peerStore {
		// Create discovery peer info from internal peer info
		discoveryPeer := types.DiscoveryPeerInfo{
			ID:           peerInfo.ID,
			Name:         peerInfo.Name,
			Addrs:        peerInfo.Addrs,
			DeviceType:   peerInfo.DeviceType,
			LastSeen:     peerInfo.LastSeen,
			Version:      peerInfo.Version,
			Groups:       peerInfo.Groups,
			Capabilities: peerInfo.Capabilities,
		}
		
		discoveryPeers[id.String()] = discoveryPeer
	}
	
	// Save to disk
	if err := n.discovery.SaveDiscoveredPeers(discoveryPeers); err != nil {
		return fmt.Errorf("failed to save peers: %w", err)
	}
	
	n.logger.Debug("Saved peers to disk", zap.Int("count", len(discoveryPeers)))
	return nil
}

// peerInfoToAddrInfo converts an InternalPeerInfo to a peer.AddrInfo
func peerInfoToAddrInfo(pi InternalPeerInfo) (peer.AddrInfo, error) {
	// Parse peer ID
	id, err := peer.Decode(pi.ID)
	if err != nil {
		return peer.AddrInfo{}, fmt.Errorf("invalid peer ID: %w", err)
	}
	
	// Parse addresses
	addrs := make([]multiaddr.Multiaddr, 0, len(pi.Addrs))
	for _, addrStr := range pi.Addrs {
		addr, err := multiaddr.NewMultiaddr(addrStr)
		if err != nil {
			// Just skip invalid addresses, don't fail
			continue
		}
		addrs = append(addrs, addr)
	}
	
	return peer.AddrInfo{
		ID:    id,
		Addrs: addrs,
	}, nil
}