# New `sync/mqtt_client.go` File Structure

This document outlines how the new `sync/mqtt_client.go` file will look after migrating from the `broker` package. This serves as a reference for developers implementing the migration.

```go
// File: internal/sync/mqtt_client.go

package sync

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
	
	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/berrythewa/clipman-daemon/pkg/utils"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// SyncClient defines the interface for synchronization functionality.
// It extends the original MQTT client interface with new synchronization capabilities.
type SyncClient interface {
	// Original MQTT functionality
	PublishContent(content *types.ClipboardContent) error
	PublishCache(cache *types.CacheMessage) error
	SubscribeToCommands() error
	RegisterCommandHandler(commandType string, handler func([]byte) error)
	IsConnected() bool
	Disconnect() error
	
	// New group functionality
	JoinGroup(groupID string) error
	LeaveGroup(groupID string) error
	ListGroups() ([]string, error)
	
	// Content filtering
	SetContentFilter(filter *ContentFilter) error
	GetContentFilter() *ContentFilter
	
	// Security features
	EnableEncryption(options *EncryptionOptions) error
	SetAuthenticationKey(key []byte) error
	
	// Discovery functions
	AnnouncePresence() error
	ListPeers() ([]PeerInfo, error)
}

// ContentFilter defines criteria for which content should be synchronized
type ContentFilter struct {
	AllowedTypes    []types.ContentType  // Content types to sync
	ExcludedTypes   []types.ContentType  // Content types to exclude
	MaxSize         int64                // Maximum size in bytes
	IncludePatterns []string             // Regex patterns to include
	ExcludePatterns []string             // Regex patterns to exclude
}

// EncryptionOptions defines settings for end-to-end encryption
type EncryptionOptions struct {
	Enabled         bool    // Whether encryption is enabled
	KeyRotationDays int     // How often to rotate keys
	Algorithm       string  // Encryption algorithm to use
}

// PeerInfo represents information about a peer device
type PeerInfo struct {
	DeviceID    string              // Unique identifier for the device
	LastSeen    time.Time           // Last time the peer was seen
	Capabilities map[string]bool    // What the peer supports
	Groups      []string            // Groups the peer belongs to
}

// SyncGroup represents a group for synchronization
type SyncGroup struct {
	ID          string     // Unique identifier for the group
	Name        string     // Display name
	Description string     // Group description
	JoinedAt    time.Time  // When this device joined the group
	Members     []string   // DeviceIDs of group members
}

// MQTTClient implements the SyncClient interface
type MQTTClient struct {
	client          mqtt.Client
	config          *config.Config
	logger          *utils.Logger
	deviceID        string
	mu              sync.Mutex
	isConnected     bool
	commandHandlers map[string]func([]byte) error
	groups          map[string]*SyncGroup
	contentFilter   *ContentFilter
	encryptionOpts  *EncryptionOptions
	peers           map[string]*PeerInfo
	// Other new fields as needed
}

// NewMQTTClient creates a new MQTT client with synchronization capabilities
func NewMQTTClient(cfg *config.Config, logger *utils.Logger) (*MQTTClient, error) {
	client := &MQTTClient{
		config:          cfg,
		logger:          logger,
		deviceID:        cfg.DeviceID,
		commandHandlers: make(map[string]func([]byte) error),
		groups:          make(map[string]*SyncGroup),
		peers:           make(map[string]*PeerInfo),
		contentFilter:   &ContentFilter{},
	}

	// If a default group is specified in config, add it
	if cfg.Sync.DefaultGroup != "" {
		client.groups[cfg.Sync.DefaultGroup] = &SyncGroup{
			ID:       cfg.Sync.DefaultGroup,
			Name:     "Default Group",
			JoinedAt: time.Now(),
		}
	}

	opts := mqtt.NewClientOptions().
		AddBroker(cfg.Broker.URL).
		SetClientID(cfg.DeviceID).
		SetUsername(cfg.Broker.Username).
		SetPassword(cfg.Broker.Password).
		SetAutoReconnect(true).
		SetOnConnectHandler(client.onConnect).
		SetConnectionLostHandler(client.onConnectionLost).
		SetReconnectingHandler(client.onReconnecting)

	client.client = mqtt.NewClient(opts)

	if err := client.connect(); err != nil {
		return nil, err
	}

	// Subscribe to presence announcements
	if err := client.subscribeToPresence(); err != nil {
		logger.Warn("Failed to subscribe to presence announcements", "error", err)
	}

	// Announce this device's presence
	if err := client.AnnouncePresence(); err != nil {
		logger.Warn("Failed to announce presence", "error", err)
	}

	return client, nil
}

// Original MQTT methods (with updates for group support)
// ...

// connect establishes a connection to the MQTT broker
func (m *MQTTClient) connect() error {
	token := m.client.Connect()
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %v", token.Error())
	}
	m.setConnected(true)
	return nil
}

// PublishContent publishes clipboard content to all joined groups
func (m *MQTTClient) PublishContent(content *types.ClipboardContent) error {
	if m == nil || m.client == nil {
		return nil // Skip publishing if no MQTT client
	}

	if !m.isConnected {
		return fmt.Errorf("not connected to MQTT broker")
	}
	
	// Apply content filtering
	if !m.shouldPublishContent(content) {
		m.logger.Debug("Content filtered out by sync filter", 
			"type", content.Type, 
			"size", len(content.Data))
		return nil
	}

	// Apply encryption if enabled
	contentToPublish := content
	if m.encryptionOpts != nil && m.encryptionOpts.Enabled {
		var err error
		contentToPublish, err = m.encryptContent(content)
		if err != nil {
			return fmt.Errorf("failed to encrypt content: %v", err)
		}
	}

	// Publish to all groups this client is a member of
	for groupID := range m.groups {
		topic := fmt.Sprintf("clipman/group/%s/content/%s", groupID, m.deviceID)
		payload, err := json.Marshal(contentToPublish)
		if err != nil {
			return fmt.Errorf("failed to marshal clipboard content: %v", err)
		}

		token := m.client.Publish(topic, 1, false, payload)
		if token.Wait() && token.Error() != nil {
			return fmt.Errorf("failed to publish content to group %s: %v", groupID, token.Error())
		}
	}

	return nil
}

// New group functionality methods

// JoinGroup joins a synchronization group
func (m *MQTTClient) JoinGroup(groupID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Check if already a member
	if _, exists := m.groups[groupID]; exists {
		return nil // Already a member
	}
	
	// Create new group entry
	m.groups[groupID] = &SyncGroup{
		ID:       groupID,
		JoinedAt: time.Now(),
	}
	
	// Subscribe to group topics
	if err := m.subscribeToGroupTopics(groupID); err != nil {
		delete(m.groups, groupID)
		return fmt.Errorf("failed to subscribe to group topics: %v", err)
	}
	
	// Announce presence to the group
	if err := m.announceToGroup(groupID); err != nil {
		m.logger.Warn("Failed to announce to group", "group", groupID, "error", err)
		// Don't fail the join operation for this
	}
	
	m.logger.Info("Joined sync group", "group", groupID)
	return nil
}

// LeaveGroup leaves a synchronization group
func (m *MQTTClient) LeaveGroup(groupID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Check if a member
	if _, exists := m.groups[groupID]; !exists {
		return nil // Not a member
	}
	
	// Unsubscribe from group topics
	if err := m.unsubscribeFromGroupTopics(groupID); err != nil {
		return fmt.Errorf("failed to unsubscribe from group topics: %v", err)
	}
	
	// Announce departure from the group
	if err := m.announceGroupDeparture(groupID); err != nil {
		m.logger.Warn("Failed to announce group departure", "group", groupID, "error", err)
		// Don't fail the leave operation for this
	}
	
	// Remove from groups map
	delete(m.groups, groupID)
	
	m.logger.Info("Left sync group", "group", groupID)
	return nil
}

// ListGroups returns all groups this client is a member of
func (m *MQTTClient) ListGroups() ([]string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	groups := make([]string, 0, len(m.groups))
	for groupID := range m.groups {
		groups = append(groups, groupID)
	}
	
	return groups, nil
}

// Content filtering methods

// SetContentFilter sets filtering criteria for content synchronization
func (m *MQTTClient) SetContentFilter(filter *ContentFilter) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.contentFilter = filter
	m.logger.Info("Content filter updated", 
		"allowed_types", filter.AllowedTypes, 
		"excluded_types", filter.ExcludedTypes,
		"max_size", filter.MaxSize)
	
	return nil
}

// GetContentFilter returns the current content filter
func (m *MQTTClient) GetContentFilter() *ContentFilter {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	return m.contentFilter
}

// shouldPublishContent checks if content passes the filter criteria
func (m *MQTTClient) shouldPublishContent(content *types.ClipboardContent) bool {
	if m.contentFilter == nil {
		return true // No filter, publish everything
	}
	
	// Check content type filters
	if len(m.contentFilter.AllowedTypes) > 0 {
		allowed := false
		for _, t := range m.contentFilter.AllowedTypes {
			if content.Type == t {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}
	
	// Check excluded types
	for _, t := range m.contentFilter.ExcludedTypes {
		if content.Type == t {
			return false
		}
	}
	
	// Check size
	if m.contentFilter.MaxSize > 0 && int64(len(content.Data)) > m.contentFilter.MaxSize {
		return false
	}
	
	// Check patterns (simplified implementation)
	// A real implementation would use regex for pattern matching
	
	return true
}

// Security-related methods

// EnableEncryption enables end-to-end encryption for sync content
func (m *MQTTClient) EnableEncryption(options *EncryptionOptions) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Initialize encryption (implementation details omitted)
	m.encryptionOpts = options
	
	m.logger.Info("Encryption enabled", 
		"algorithm", options.Algorithm,
		"key_rotation_days", options.KeyRotationDays)
	
	return nil
}

// SetAuthenticationKey sets the authentication key for secure communication
func (m *MQTTClient) SetAuthenticationKey(key []byte) error {
	// Implementation details omitted
	return nil
}

// Discovery-related methods

// AnnouncePresence announces this device's presence to other peers
func (m *MQTTClient) AnnouncePresence() error {
	// Create a presence message
	presence := struct {
		DeviceID     string              `json:"device_id"`
		Timestamp    int64               `json:"timestamp"`
		Capabilities map[string]bool     `json:"capabilities"`
		Groups       []string            `json:"groups"`
	}{
		DeviceID:  m.deviceID,
		Timestamp: time.Now().UnixNano(),
		Capabilities: map[string]bool{
			"text":      true,
			"image":     true,
			"file":      true,
			"encrypted": m.encryptionOpts != nil && m.encryptionOpts.Enabled,
		},
		Groups: make([]string, 0, len(m.groups)),
	}
	
	// Add groups
	for groupID := range m.groups {
		presence.Groups = append(presence.Groups, groupID)
	}
	
	// Publish presence message
	payload, err := json.Marshal(presence)
	if err != nil {
		return fmt.Errorf("failed to marshal presence: %v", err)
	}
	
	topic := fmt.Sprintf("clipman/presence/%s", m.deviceID)
	token := m.client.Publish(topic, 1, false, payload)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish presence: %v", token.Error())
	}
	
	return nil
}

// ListPeers returns information about known peers
func (m *MQTTClient) ListPeers() ([]PeerInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	peers := make([]PeerInfo, 0, len(m.peers))
	for _, peer := range m.peers {
		peers = append(peers, *peer)
	}
	
	return peers, nil
}

// Helper methods

// subscribeToGroupTopics subscribes to all topics for a specific group
func (m *MQTTClient) subscribeToGroupTopics(groupID string) error {
	// Subscribe to content
	contentTopic := fmt.Sprintf("clipman/group/%s/content/+", groupID)
	if token := m.client.Subscribe(contentTopic, 1, m.handleGroupContent); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to subscribe to group content: %v", token.Error())
	}
	
	// Subscribe to cache
	cacheTopic := fmt.Sprintf("clipman/group/%s/cache/+", groupID)
	if token := m.client.Subscribe(cacheTopic, 1, m.handleGroupCache); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to subscribe to group cache: %v", token.Error())
	}
	
	// Subscribe to commands
	commandsTopic := fmt.Sprintf("clipman/group/%s/commands/%s", groupID, m.deviceID)
	if token := m.client.Subscribe(commandsTopic, 1, m.handleCommand); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to subscribe to group commands: %v", token.Error())
	}
	
	// Subscribe to broadcast commands
	broadcastTopic := fmt.Sprintf("clipman/group/%s/commands/broadcast", groupID)
	if token := m.client.Subscribe(broadcastTopic, 1, m.handleCommand); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to subscribe to broadcast commands: %v", token.Error())
	}
	
	return nil
}

// unsubscribeFromGroupTopics unsubscribes from all topics for a specific group
func (m *MQTTClient) unsubscribeFromGroupTopics(groupID string) error {
	// Implementation details omitted for brevity
	return nil
}

// handleGroupContent processes content received from a group
func (m *MQTTClient) handleGroupContent(client mqtt.Client, msg mqtt.Message) {
	// Implementation details omitted for brevity
}

// handleGroupCache processes cache messages received from a group
func (m *MQTTClient) handleGroupCache(client mqtt.Client, msg mqtt.Message) {
	// Implementation details omitted for brevity
}

// subscribeToPresence subscribes to peer presence announcements
func (m *MQTTClient) subscribeToPresence() error {
	topic := "clipman/presence/+"
	if token := m.client.Subscribe(topic, 1, m.handlePresence); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to subscribe to presence: %v", token.Error())
	}
	return nil
}

// handlePresence processes presence announcements from peers
func (m *MQTTClient) handlePresence(client mqtt.Client, msg mqtt.Message) {
	// Implementation details omitted for brevity
}

// Other event handlers and helper methods...
```

## Configuration Changes

The `config` package needs to be updated to support the new sync package:

```go
// File: internal/config/config.go

// SyncConfig defines synchronization-related configuration
type SyncConfig struct {
	// Default synchronization group
	DefaultGroup string `json:"default_group"`
	
	// Whether to automatically join discovered groups
	AutoJoinGroups bool `json:"auto_join_groups"`
	
	// Content filtering options
	AllowedTypes    []string `json:"allowed_types"`
	ExcludedTypes   []string `json:"excluded_types"`
	MaxSyncSize     int64    `json:"max_sync_size"`
	IncludePatterns []string `json:"include_patterns"`
	ExcludePatterns []string `json:"exclude_patterns"`
	
	// Security options
	EnableEncryption bool   `json:"enable_encryption"`
	EncryptionAlgo   string `json:"encryption_algo"`
	KeyRotationDays  int    `json:"key_rotation_days"`
	
	// Discovery options
	AnnounceInterval int `json:"announce_interval"`
	PeerTimeout      int `json:"peer_timeout"`
}

// Update the main Config struct to include SyncConfig
type Config struct {
	// Existing fields...
	
	// Sync configuration (replaces Broker)
	Sync SyncConfig `json:"sync"`
	
	// Keep Broker for backward compatibility
	Broker BrokerConfig `json:"broker"`
}
```

## Usage Example

Here's how the new sync package would be used:

```go
// Initialize the sync client
cfg, _ := config.Load()
logger := utils.NewLogger(utils.LoggerOptions{Level: "info"})
syncClient, err := sync.NewMQTTClient(cfg, logger)
if err != nil {
	log.Fatalf("Failed to initialize sync client: %v", err)
}

// Join a sync group
if err := syncClient.JoinGroup("work-devices"); err != nil {
	log.Printf("Failed to join group: %v", err)
}

// Set content filtering
filter := &sync.ContentFilter{
	AllowedTypes:  []types.ContentType{types.TypeText, types.TypeURL},
	MaxSize:       1024 * 1024, // 1MB
	ExcludePatterns: []string{"password=.*", "token=.*"},
}
syncClient.SetContentFilter(filter)

// Enable encryption
encOpts := &sync.EncryptionOptions{
	Enabled:         true,
	Algorithm:       "AES-GCM",
	KeyRotationDays: 7,
}
syncClient.EnableEncryption(encOpts)

// Publish content
content := &types.ClipboardContent{
	Type:    types.TypeText,
	Data:    []byte("Hello, secure sync!"),
	Created: time.Now(),
}
syncClient.PublishContent(content)