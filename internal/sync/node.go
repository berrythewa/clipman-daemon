// Package sync provides clipboard synchronization using libp2p
package sync

import (
	"context"
	"crypto/rand"
	"fmt"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/p2p/discovery/mdns"
	"github.com/libp2p/go-libp2p/p2p/host/eventbus"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"go.uber.org/zap"
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
	mdnsService   mdns.Service
	
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
	
	// Initialize discovery
	if err := n.setupDiscovery(); err != nil {
		return fmt.Errorf("failed to set up discovery: %w", err)
	}
	
	// Initialize pubsub if needed
	if err := n.setupPubSub(); err != nil {
		return fmt.Errorf("failed to set up pubsub: %w", err)
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
	
	// Stop any services
	if n.mdnsService != nil {
		n.mdnsService.Close()
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

// setupDiscovery sets up peer discovery mechanisms
func (n *Node) setupDiscovery() error {
	// Get discovery config
	discCfg := GetDiscoveryConfig(n.config)
	
	// Set up mDNS discovery if enabled
	if discCfg.EnableMDNS {
		service := mdns.NewMdnsService(n.host, "clipman", &discoveryNotifee{node: n})
		if err := service.Start(); err != nil {
			return fmt.Errorf("failed to start mDNS service: %w", err)
		}
		n.mdnsService = service
		n.logger.Info("Started mDNS discovery service")
	}
	
	// Set up DHT discovery if enabled (not implemented yet)
	if discCfg.EnableDHT {
		n.logger.Warn("DHT discovery requested but not implemented yet")
	}
	
	return nil
}

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

// discoveryNotifee gets notified when we discover a new peer
type discoveryNotifee struct {
	node *Node
}

// HandlePeerFound is called when a peer is discovered
func (n *discoveryNotifee) HandlePeerFound(pi peer.AddrInfo) {
	// Ignore our own peer ID
	if pi.ID == n.node.host.ID() {
		return
	}
	
	n.node.logger.Debug("Discovered peer via mDNS", zap.String("peer_id", pi.ID.String()))
	
	// Add peer to host peerstore and connect
	n.node.host.Peerstore().AddAddrs(pi.ID, pi.Addrs, time.Hour)
	
	// Attempt to connect to the peer
	ctx, cancel := context.WithTimeout(n.node.ctx, time.Second*10)
	defer cancel()
	
	if err := n.node.host.Connect(ctx, pi); err != nil {
		n.node.logger.Warn("Failed to connect to discovered peer", 
			zap.String("peer_id", pi.ID.String()),
			zap.Error(err))
		return
	}
	
	// Create PeerInfo and add to our store
	peerInfo := InternalPeerInfo{
		ID:           pi.ID.String(),
		LastSeen:     time.Now(),
		Capabilities: make(map[string]string),
		// Other fields would be populated later after protocol handshake
	}
	
	n.node.AddPeer(peerInfo)
}