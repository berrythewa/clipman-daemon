// Package sync provides synchronization capabilities for clipman
// This file implements the SyncManager interface
package sync

import (
	"fmt"
	"sync"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/sync/discovery"
	"github.com/berrythewa/clipman-daemon/internal/sync/protocol"
	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Manager implements the SyncManager interface
type Manager struct {
	config          *config.Config
	logger          *zap.Logger
	clientID        string
	client          protocol.Client
	mode            SyncMode
	status          SyncStatus
	statusMu        sync.RWMutex
	messageHandlers []MessageHandler
	contentHandlers []ContentHandlerFunc
	handlerMu       sync.RWMutex
	stats           *SyncStats
	statsMu         sync.RWMutex
	discovery       Discovery          // Discovery mechanism
	discoveryMu     sync.RWMutex       // Mutex for discovery operations
	peers           map[string]PeerInfo // Discovered peers
	peersMu         sync.RWMutex       // Mutex for peers map
	deviceName      string             // Local device name
}

// NewManager creates a new sync manager
func NewManager(cfg *config.Config, mode SyncMode) (*Manager, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	// Use a default logger if none provided
	logger := zap.NewNop()
	
	// Generate unique client ID if not specified
	clientID := cfg.Sync.ClientID
	if clientID == "" {
		clientID = uuid.New().String()
	}

	// Create protocol client
	protocolName := cfg.Sync.Protocol
	if protocolName == "" {
		protocolName = string(mode) // Use the mode as the protocol name
	}

	// Create base protocol options
	protocolOpts := &protocol.ProtocolOptions{
		ReconnectDelay:    5 * time.Second,
		ReconnectMaxRetry: 12, // Try for 1 minute
	}

	// Get protocol factory
	factory, err := protocol.GetProtocolFactory(protocolName)
	if err != nil {
		return nil, fmt.Errorf("failed to get protocol factory: %w", err)
	}

	// Create client
	client, err := factory.NewClient(protocolOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create protocol client: %w", err)
	}

	// Get device name
	deviceName := cfg.DeviceName
	if deviceName == "" {
		hostName, err := getHostname()
		if err != nil {
			deviceName = "Unknown Device"
		} else {
			deviceName = hostName
		}
	}

	mgr := &Manager{
		config:          cfg,
		logger:          logger.With(zap.String("component", "sync_manager")),
		clientID:        clientID,
		client:          client,
		mode:            mode,
		status:          SyncStatusDisconnected,
		messageHandlers: make([]MessageHandler, 0),
		contentHandlers: make([]ContentHandlerFunc, 0),
		stats:           &SyncStats{LastSyncTime: time.Now()},
		peers:           make(map[string]PeerInfo),
		deviceName:      deviceName,
	}

	// Initialize discovery if enabled in config
	if cfg.Sync.EnableDiscovery {
		if err := mgr.initDiscovery(); err != nil {
			logger.Warn("Failed to initialize discovery", zap.Error(err))
		}
	}

	return mgr, nil
}

// getHostname gets the system hostname
func getHostname() (string, error) {
	// Use os.Hostname(), but we'll simulate it here
	return "clipman-device", nil
}

// SetLogger sets the logger for the manager
func (m *Manager) SetLogger(logger *zap.Logger) {
	if logger != nil {
		m.logger = logger.With(zap.String("component", "sync_manager"))
		
		// Update logger for discovery too
		m.discoveryMu.RLock()
		disc := m.discovery
		m.discoveryMu.RUnlock()
		
		if disc != nil {
			// Some discovery implementations might provide SetLogger
			if discLogger, ok := disc.(interface {
				SetLogger(*zap.Logger)
			}); ok {
				discLogger.SetLogger(logger.With(zap.String("component", "discovery")))
			}
		}
	}
}

// Start initializes and starts the sync manager
func (m *Manager) Start() error {
	m.logger.Info("Starting sync manager", 
		zap.String("mode", string(m.mode)),
		zap.String("client_id", m.clientID))
	
	// Start discovery if available
	if m.config.Sync.EnableDiscovery {
		m.discoveryMu.RLock()
		disc := m.discovery
		m.discoveryMu.RUnlock()
		
		if disc != nil {
			m.logger.Info("Starting discovery service")
			if err := disc.Start(); err != nil {
				m.logger.Error("Failed to start discovery", zap.Error(err))
			}
			
			// Announce our presence
			m.announceSelf()
		}
	}
	
	return m.Connect()
}

// Stop stops the sync manager and releases resources
func (m *Manager) Stop() error {
	m.logger.Info("Stopping sync manager")
	
	// Stop discovery first
	m.discoveryMu.RLock()
	disc := m.discovery
	m.discoveryMu.RUnlock()
	
	if disc != nil {
		m.logger.Info("Stopping discovery service")
		if err := disc.Stop(); err != nil {
			m.logger.Error("Failed to stop discovery", zap.Error(err))
		}
	}
	
	return m.Disconnect()
}

// Status returns the current sync status
func (m *Manager) Status() SyncStatus {
	m.statusMu.RLock()
	defer m.statusMu.RUnlock()
	
	// Create a status struct with current information
	activeGroups := m.client.ListGroups()
	
	m.statsMu.RLock()
	stats := *m.stats // Make a copy of the stats
	m.statsMu.RUnlock()
	
	m.peersMu.RLock()
	peerCount := len(m.peers)
	m.peersMu.RUnlock()
	
	return SyncStatus{
		Connected:    m.status == SyncStatusConnected,
		Mode:         string(m.mode),
		ActiveGroups: activeGroups,
		PeerCount:    peerCount,
		Stats:        stats,
	}
}

// Connect connects to the sync backend
func (m *Manager) Connect() error {
	m.statusMu.Lock()
	defer m.statusMu.Unlock()

	if m.status == SyncStatusConnected || m.status == SyncStatusConnecting {
		return nil
	}

	// Update status
	m.status = SyncStatusConnecting
	m.logger.Info("Connecting to sync backend", zap.String("protocol", m.config.Sync.Protocol))

	// Connect to the protocol client
	if err := m.client.Connect(); err != nil {
		m.status = SyncStatusError
		m.logger.Error("Failed to connect to sync backend", zap.Error(err))
		return fmt.Errorf("failed to connect: %w", err)
	}

	// Register message handler
	m.client.AddHandler(m.handleMessage)

	// Join configured groups
	defaultGroup := m.config.Sync.DefaultGroup
	if defaultGroup != "" {
		m.logger.Info("Joining default group", zap.String("group", defaultGroup))
		if err := m.client.JoinGroup(defaultGroup); err != nil {
			m.logger.Error("Failed to join default group", 
				zap.String("group", defaultGroup), 
				zap.Error(err))
		}
	}

	// Update status
	m.status = SyncStatusConnected
	m.logger.Info("Connected to sync backend")

	return nil
}

// Disconnect disconnects from the sync backend
func (m *Manager) Disconnect() error {
	m.statusMu.Lock()
	defer m.statusMu.Unlock()

	if m.status == SyncStatusDisconnected {
		return nil
	}

	m.logger.Info("Disconnecting from sync backend")
	
	// Disconnect from the protocol client
	if err := m.client.Disconnect(); err != nil {
		m.logger.Error("Error disconnecting from sync backend", zap.Error(err))
		return fmt.Errorf("failed to disconnect: %w", err)
	}

	// Update status
	m.status = SyncStatusDisconnected
	m.logger.Info("Disconnected from sync backend")

	return nil
}

// IsConnected returns whether the manager is connected
func (m *Manager) IsConnected() bool {
	m.statusMu.RLock()
	defer m.statusMu.RUnlock()

	return m.status == SyncStatusConnected && m.client.IsConnected()
}

// SendContent sends clipboard content to the specified group
func (m *Manager) SendContent(content *types.ClipboardContent, group string) error {
	if !m.IsConnected() {
		return fmt.Errorf("not connected to sync backend")
	}

	if content == nil {
		return fmt.Errorf("content cannot be nil")
	}

	m.logger.Debug("Sending content", 
		zap.String("type", string(content.Type)),
		zap.Int("size", len(content.Data)),
		zap.String("group", group))

	// Use our internal publishing method
	return m.PublishContent(content, group)
}

// PublishContent publishes clipboard content to the specified group
func (m *Manager) PublishContent(content *types.ClipboardContent, group string) error {
	if !m.IsConnected() {
		return fmt.Errorf("not connected to sync backend")
	}

	if content == nil {
		return fmt.Errorf("content cannot be nil")
	}

	// Ensure content has Group field set
	if content.Group == "" {
		content.Group = group
	}

	// Create a content message using the protocol factory
	factory, err := protocol.GetProtocolFactory(m.config.Sync.Protocol)
	if err != nil {
		return fmt.Errorf("failed to get protocol factory: %w", err)
	}

	// Create a message that matches our protocol
	msg, err := protocol.CreateContentMessage(factory, content)
	if err != nil {
		return fmt.Errorf("failed to create content message: %w", err)
	}

	// Set message properties
	msg.SetSource(m.clientID)
	msg.SetGroup(group)

	// Track stats
	startTime := time.Now()
	
	// Send the message
	err = m.client.Send(msg)
	
	// Update stats
	m.updateSendStats(len(content.Data), err == nil, time.Since(startTime))
	
	if err != nil {
		m.logger.Error("Failed to send content", 
			zap.String("group", group),
			zap.Error(err))
		return fmt.Errorf("failed to send content: %w", err)
	}
	
	m.logger.Debug("Content sent successfully", 
		zap.String("group", group),
		zap.String("content_type", string(content.Type)))
		
	return nil
}

// SetContentHandler sets the handler for received clipboard content
func (m *Manager) SetContentHandler(handler func(*types.ClipboardContent, PeerInfo)) {
	m.handlerMu.Lock()
	defer m.handlerMu.Unlock()
	
	m.contentHandlers = []ContentHandlerFunc{handler}
	m.logger.Debug("Content handler set")
}

// AddContentHandler adds a handler for received clipboard content
func (m *Manager) AddContentHandler(handler func(*types.ClipboardContent, PeerInfo)) {
	m.handlerMu.Lock()
	defer m.handlerMu.Unlock()
	
	m.contentHandlers = append(m.contentHandlers, handler)
	m.logger.Debug("Content handler added", 
		zap.Int("total_handlers", len(m.contentHandlers)))
}

// JoinGroup joins a synchronization group
func (m *Manager) JoinGroup(group string) error {
	if group == "" {
		return fmt.Errorf("group name cannot be empty")
	}

	m.logger.Info("Joining group", zap.String("group", group))
	return m.client.JoinGroup(group)
}

// LeaveGroup leaves a synchronization group
func (m *Manager) LeaveGroup(group string) error {
	if group == "" {
		return fmt.Errorf("group name cannot be empty")
	}

	m.logger.Info("Leaving group", zap.String("group", group))
	return m.client.LeaveGroup(group)
}

// ListGroups returns the list of joined groups
func (m *Manager) ListGroups() ([]string, error) {
	groups := m.client.ListGroups()
	m.logger.Debug("Listing groups", zap.Strings("groups", groups))
	return groups, nil
}

// initDiscovery initializes the discovery mechanism
func (m *Manager) initDiscovery() error {
	m.discoveryMu.Lock()
	defer m.discoveryMu.Unlock()
	
	// Skip if already initialized
	if m.discovery != nil {
		return nil
	}
	
	// Get discovery method from config
	method := m.config.Sync.DiscoveryMethod
	if method == "" {
		// Default to mDNS for local discovery
		method = string(discovery.MethodMDNS)
	}
	
	// Set up discovery options
	options := &discovery.DiscoveryOptions{
		AnnounceInterval: 60,                 // Announce every minute
		PeerTimeout:      120,                // Peers timeout after 2 minutes
		DeviceID:         m.clientID,         // Use our client ID
		DeviceName:       m.deviceName,       // Use our device name
		Logger:           m.logger,           // Use our logger
	}
	
	// Get factory for the specified method
	factory, err := discovery.GetDiscoveryFactory(method)
	if err != nil {
		return fmt.Errorf("failed to get discovery factory: %w", err)
	}
	
	// Create discovery instance
	disc, err := factory.CreateDiscovery(options)
	if err != nil {
		return fmt.Errorf("failed to create discovery: %w", err)
	}
	
	// Set handler for discovered peers
	disc.SetPeerDiscoveryHandler(m.handlePeerDiscovered)
	
	// Store discovery instance
	m.discovery = disc
	
	m.logger.Info("Discovery initialized", zap.String("method", method))
	
	return nil
}

// EnableDiscovery enables or disables peer discovery
func (m *Manager) EnableDiscovery(enabled bool) error {
	m.logger.Info("Discovery mode changed", zap.Bool("enabled", enabled))
	
	// Update config
	m.config.Sync.EnableDiscovery = enabled
	
	if enabled {
		// Initialize discovery if not already
		if err := m.initDiscovery(); err != nil {
			return fmt.Errorf("failed to initialize discovery: %w", err)
		}
		
		// Start discovery
		m.discoveryMu.RLock()
		disc := m.discovery
		m.discoveryMu.RUnlock()
		
		if disc != nil {
			if err := disc.Start(); err != nil {
				return fmt.Errorf("failed to start discovery: %w", err)
			}
			
			// Announce our presence
			m.announceSelf()
		}
	} else {
		// Stop discovery
		m.discoveryMu.RLock()
		disc := m.discovery
		m.discoveryMu.RUnlock()
		
		if disc != nil {
			if err := disc.Stop(); err != nil {
				return fmt.Errorf("failed to stop discovery: %w", err)
			}
		}
	}
	
	return nil
}

// GetDiscoveredPeers returns the list of discovered peers
func (m *Manager) GetDiscoveredPeers() []PeerInfo {
	m.peersMu.RLock()
	defer m.peersMu.RUnlock()
	
	// Convert map to slice
	peers := make([]PeerInfo, 0, len(m.peers))
	for _, peer := range m.peers {
		peers = append(peers, peer)
	}
	
	return peers
}

// announceSelf announces our presence to other peers
func (m *Manager) announceSelf() {
	m.discoveryMu.RLock()
	disc := m.discovery
	m.discoveryMu.RUnlock()
	
	if disc == nil {
		return
	}
	
	// Get our groups
	groups := m.client.ListGroups()
	
	// Create peer info
	peer := PeerInfo{
		ID:           m.clientID,
		Name:         m.deviceName,
		Groups:       groups,
		LastSeen:     time.Now(),
		Capabilities: map[string]string{
			"version":  "1.0.0",
			"protocol": string(m.mode),
		},
		Version:    "1.0.0",
		DeviceType: "desktop",
	}
	
	// Announce our presence
	if err := disc.Announce(peer); err != nil {
		m.logger.Error("Failed to announce presence", zap.Error(err))
	}
}

// handlePeerDiscovered handles discovered peers
func (m *Manager) handlePeerDiscovered(peer PeerInfo) {
	// Skip our own announcements
	if peer.ID == m.clientID {
		return
	}
	
	m.logger.Debug("Peer discovered", 
		zap.String("peer_id", peer.ID),
		zap.String("peer_name", peer.Name),
		zap.Strings("groups", peer.Groups))
	
	// Add peer to our list
	m.peersMu.Lock()
	m.peers[peer.ID] = peer
	m.peersMu.Unlock()
}

// AddHandler adds a message handler
func (m *Manager) AddHandler(handler MessageHandler) {
	m.handlerMu.Lock()
	defer m.handlerMu.Unlock()

	m.messageHandlers = append(m.messageHandlers, handler)
	m.logger.Debug("Message handler added", 
		zap.Int("total_handlers", len(m.messageHandlers)))
}

// updateSendStats updates stats after sending a message
func (m *Manager) updateSendStats(bytes int, success bool, duration time.Duration) {
	m.statsMu.Lock()
	defer m.statsMu.Unlock()
	
	m.stats.MessagesSent++
	m.stats.BytesSent += int64(bytes)
	m.stats.LastSyncTime = time.Now()
	
	if !success {
		m.stats.Errors++
	}
	
	// Update success rate
	total := m.stats.MessagesSent + m.stats.MessagesReceived
	if total > 0 {
		successCount := total - m.stats.Errors
		m.stats.SuccessRate = float64(successCount) / float64(total) * 100.0
	}
}

// updateReceiveStats updates stats after receiving a message
func (m *Manager) updateReceiveStats(bytes int, success bool) {
	m.statsMu.Lock()
	defer m.statsMu.Unlock()
	
	m.stats.MessagesReceived++
	m.stats.BytesReceived += int64(bytes)
	m.stats.LastSyncTime = time.Now()
	
	if !success {
		m.stats.Errors++
	}
	
	// Update success rate
	total := m.stats.MessagesSent + m.stats.MessagesReceived
	if total > 0 {
		successCount := total - m.stats.Errors
		m.stats.SuccessRate = float64(successCount) / float64(total) * 100.0
	}
}

// handleMessage processes an incoming message
func (m *Manager) handleMessage(msg Message) {
	// Skip self-originated messages to avoid loops
	if msg.Source() == m.clientID {
		m.logger.Debug("Skipping self-originated message")
		return
	}

	m.logger.Debug("Received message",
		zap.String("type", msg.Type()),
		zap.String("source", msg.Source()),
		zap.String("group", msg.Group()),
		zap.Time("timestamp", msg.Timestamp()))

	// Process message based on type
	switch msg.Type() {
	case string(protocol.MessageTypeContent):
		m.handleContentMessage(msg)
	default:
		// Handle other message types with registered handlers
		m.dispatchToHandlers(msg)
	}
	
	// Update receive stats (basic size estimate based on payload)
	m.updateReceiveStats(len(msg.Payload()), true)
}

// handleContentMessage handles content-specific messages
func (m *Manager) handleContentMessage(msg Message) {
	// Try to extract clipboard content
	// This is a bit hacky - in a full implementation we'd have a cleaner way to do this
	contentMsg, ok := msg.(interface {
		GetContent() *types.ClipboardContent
	})
	
	if !ok {
		m.logger.Error("Failed to extract content from message")
		return
	}
	
	content := contentMsg.GetContent()
	if content == nil {
		m.logger.Error("Content message contained nil content")
		return
	}
	
	// Create peer info
	peer := PeerInfo{
		ID:       msg.Source(),
		LastSeen: time.Now(),
		Groups:   []string{msg.Group()},
	}
	
	// Update peer info if we have more details from discovery
	m.peersMu.RLock()
	if discoveredPeer, ok := m.peers[msg.Source()]; ok {
		peer.Name = discoveredPeer.Name
		peer.Address = discoveredPeer.Address
		peer.Capabilities = discoveredPeer.Capabilities
		peer.Version = discoveredPeer.Version
		peer.DeviceType = discoveredPeer.DeviceType
	}
	m.peersMu.RUnlock()
	
	// Call content handlers
	m.handlerMu.RLock()
	handlers := make([]ContentHandlerFunc, len(m.contentHandlers))
	copy(handlers, m.contentHandlers)
	m.handlerMu.RUnlock()
	
	m.logger.Debug("Dispatching content to handlers",
		zap.Int("handler_count", len(handlers)),
		zap.String("content_type", string(content.Type)))
		
	for _, handler := range handlers {
		if handler != nil {
			go handler(content, peer)
		}
	}
}

// dispatchToHandlers dispatches a message to all registered message handlers
func (m *Manager) dispatchToHandlers(msg Message) {
	// Get handlers
	m.handlerMu.RLock()
	handlers := make([]MessageHandler, len(m.messageHandlers))
	copy(handlers, m.messageHandlers)
	m.handlerMu.RUnlock()

	// Call all handlers
	for _, handler := range handlers {
		if handler != nil {
			go handler(msg)
		}
	}
} 