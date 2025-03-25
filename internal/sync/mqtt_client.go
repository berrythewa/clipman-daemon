// Package sync provides synchronization capabilities between Clipman instances
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
	"github.com/sirupsen/logrus"
	"go.uber.org/zap"
)

// SyncClient defines the interface for synchronization functionality.
// It maintains compatibility with the original broker.MQTTClientInterface
// while adding new synchronization capabilities.
type SyncClient interface {
	// Original broker functionality
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
}

// ContentFilter defines criteria for which content should be synchronized
type ContentFilter struct {
	AllowedTypes    []types.ContentType  // Content types to sync
	ExcludedTypes   []types.ContentType  // Content types to exclude
	MaxSize         int64                // Maximum size in bytes
	IncludePatterns []string             // Regex patterns to include
	ExcludePatterns []string             // Regex patterns to exclude
}

// SyncGroup represents a group for synchronization
type SyncGroup struct {
	ID          string     // Unique identifier for the group
	Name        string     // Display name
	Description string     // Group description
	JoinedAt    time.Time  // When this device joined the group
	Members     []string   // DeviceIDs of group members
}

// Group management constants
const (
	TopicGroups     = "groups"
	TopicGroupJoin  = "join"
	TopicGroupLeave = "leave"
)

// Additional methods for MQTTClientInterface to support CLI functionality
type MQTTClientInterface interface {
	Connect() error
	Disconnect()
	Publish(topic string, payload interface{}) error
	Subscribe(topic string, callback types.MessageCallback) error
	Unsubscribe(topic string) error
	
	// Group management methods
	JoinGroup(groupName string) error
	LeaveGroup(groupName string) error
	ListGroups() ([]string, error)
	
	// Connection status
	IsConnected() bool
}

// MQTTClient implements MQTTClientInterface
type MQTTClient struct {
	client          mqtt.Client
	config          *config.Config
	logger          *zap.Logger
	deviceID        string
	mu              sync.Mutex
	connected       bool
	commandHandlers map[string]func([]byte) error
	groups          map[string]*SyncGroup
	contentFilter   *ContentFilter
	syncMode        string
}

// CreateClient creates a new SyncClient with the appropriate mode
func CreateClient(cfg *config.Config, logger *zap.Logger) (SyncClient, error) {
	// Ensure config compatibility
	if cfg.Sync.URL == "" && cfg.Broker.URL != "" {
		cfg.Sync.URL = cfg.Broker.URL
		cfg.Sync.Username = cfg.Broker.Username
		cfg.Sync.Password = cfg.Broker.Password
	}

	// Create appropriate client based on mode
	if cfg.Sync.IsModeCentralized() {
		// Future: return NewCentralizedClient(cfg, logger)
		return nil, fmt.Errorf("centralized sync mode not yet implemented")
	} else {
		// Default to P2P mode
		return NewMQTTClient(cfg, logger)
	}
}

// NewMQTTClient creates a new MQTT client with synchronization capabilities
func NewMQTTClient(cfg *config.Config, logger *zap.Logger) (*MQTTClient, error) {
	client := &MQTTClient{
		config:          cfg,
		logger:          logger,
		deviceID:        cfg.DeviceID,
		commandHandlers: make(map[string]func([]byte) error),
		groups:          make(map[string]*SyncGroup),
		contentFilter:   &ContentFilter{},
		syncMode:        cfg.Sync.Mode,
	}

	// Use either Sync or Broker config, preferring Sync
	brokerURL := cfg.Sync.URL
	brokerUsername := cfg.Sync.Username
	brokerPassword := cfg.Sync.Password
	
	if brokerURL == "" {
		brokerURL = cfg.Broker.URL
		brokerUsername = cfg.Broker.Username
		brokerPassword = cfg.Broker.Password
	}

	// If a default group is specified, add it
	if cfg.Sync.DefaultGroup != "" {
		client.groups[cfg.Sync.DefaultGroup] = &SyncGroup{
			ID:       cfg.Sync.DefaultGroup,
			Name:     "Default Group",
			JoinedAt: time.Now(),
		}
	}

	// Setup MQTT client
	opts := mqtt.NewClientOptions().
		AddBroker(brokerURL).
		SetClientID(cfg.DeviceID).
		SetUsername(brokerUsername).
		SetPassword(brokerPassword).
		SetAutoReconnect(true).
		SetOnConnectHandler(client.onConnect).
		SetConnectionLostHandler(client.onConnectionLost).
		SetReconnectingHandler(client.onReconnecting)

	client.client = mqtt.NewClient(opts)

	if err := client.connect(); err != nil {
		return nil, err
	}

	return client, nil
}

// connect establishes a connection to the MQTT broker
func (m *MQTTClient) connect() error {
	token := m.client.Connect()
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %v", token.Error())
	}
	m.setConnected(true)
	return nil
}

// PublishContent publishes clipboard content
func (m *MQTTClient) PublishContent(content *types.ClipboardContent) error {
	if m == nil || m.client == nil {
		return nil // Skip publishing if no MQTT client
	}

	if !m.connected {
		return fmt.Errorf("not connected to MQTT broker")
	}

	// Check content against filters if filtering is enabled
	if m.contentFilter != nil && !m.shouldPublishContent(content) {
		m.logger.Debug("Content filtered out by sync filter", 
			"type", content.Type, 
			"size", len(content.Data))
		return nil
	}

	// Determine if we should use group-based topics
	useGroups := len(m.groups) > 0

	if useGroups {
		// Publish to all groups this client is a member of
		for groupID := range m.groups {
			topic := fmt.Sprintf("clipman/group/%s/content/%s", groupID, m.deviceID)
			payload, err := json.Marshal(content)
			if err != nil {
				return fmt.Errorf("failed to marshal clipboard content: %v", err)
			}
	
			token := m.client.Publish(topic, 1, false, payload)
			if token.Wait() && token.Error() != nil {
				return fmt.Errorf("failed to publish content to group %s: %v", groupID, token.Error())
			}
		}
	} else {
		// Fallback to original topic structure for backward compatibility
		topic := fmt.Sprintf("clipman/%s/content", m.deviceID)
		payload, err := json.Marshal(content)
		if err != nil {
			return fmt.Errorf("failed to marshal clipboard content: %v", err)
		}
	
		token := m.client.Publish(topic, 1, false, payload)
		if token.Wait() && token.Error() != nil {
			return fmt.Errorf("failed to publish content: %v", token.Error())
		}
	}
	
	return nil
}

// PublishCache publishes cache information
func (m *MQTTClient) PublishCache(cache *types.CacheMessage) error {
	if m == nil || m.client == nil {
		return nil // Skip publishing if no MQTT client
	}

	if !m.connected {
		return fmt.Errorf("not connected to MQTT broker")
	}

	// Determine if we should use group-based topics
	useGroups := len(m.groups) > 0

	if useGroups {
		// Publish to all groups this client is a member of
		for groupID := range m.groups {
			topic := fmt.Sprintf("clipman/group/%s/cache/%s", groupID, cache.DeviceID)
			payload, err := json.Marshal(cache)
			if err != nil {
				return fmt.Errorf("failed to marshal cache: %v", err)
			}
		
			token := m.client.Publish(topic, 1, false, payload)
			if token.Wait() && token.Error() != nil {
				return fmt.Errorf("failed to publish cache to group %s: %v", groupID, token.Error())
			}
		}
	} else {
		// Fallback to original topic structure for backward compatibility
		topic := fmt.Sprintf("clipman/cache/%s", cache.DeviceID)
		payload, err := json.Marshal(cache)
		if err != nil {
			return fmt.Errorf("failed to marshal cache: %v", err)
		}
	
		token := m.client.Publish(topic, 1, false, payload)
		if token.Wait() && token.Error() != nil {
			return fmt.Errorf("failed to publish cache: %v", token.Error())
		}
	}
	
	return nil
}

// SubscribeToCommands subscribes to command messages
func (m *MQTTClient) SubscribeToCommands() error {
	if !m.connected {
		return fmt.Errorf("not connected to MQTT broker")
	}

	// Subscribe to device-specific commands (always do this for backward compatibility)
	topic := fmt.Sprintf("clipman/%s/commands", m.deviceID)
	token := m.client.Subscribe(topic, 1, m.handleCommand)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to subscribe to commands: %v", token.Error())
	}

	// If we have groups, also subscribe to group command topics
	if len(m.groups) > 0 {
		for groupID := range m.groups {
			// Subscribe to direct group commands
			groupTopic := fmt.Sprintf("clipman/group/%s/commands/%s", groupID, m.deviceID)
			token := m.client.Subscribe(groupTopic, 1, m.handleCommand)
			if token.Wait() && token.Error() != nil {
				m.logger.Warn("Failed to subscribe to group commands", 
					"group", groupID, "error", token.Error())
				continue
			}
			
			// Subscribe to broadcast group commands
			broadcastTopic := fmt.Sprintf("clipman/group/%s/commands/broadcast", groupID)
			token = m.client.Subscribe(broadcastTopic, 1, m.handleCommand)
			if token.Wait() && token.Error() != nil {
				m.logger.Warn("Failed to subscribe to broadcast commands", 
					"group", groupID, "error", token.Error())
			}
		}
	}
	
	return nil
}

// handleCommand processes incoming command messages
func (m *MQTTClient) handleCommand(client mqtt.Client, msg mqtt.Message) {
	var command struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	}

	if err := json.Unmarshal(msg.Payload(), &command); err != nil {
		m.logger.Error("Failed to unmarshal command", "error", err)
		return
	}

	handler, ok := m.commandHandlers[command.Type]
	if !ok {
		m.logger.Warn("Unknown command type", "type", command.Type)
		return
	}

	if err := handler(command.Data); err != nil {
		m.logger.Error("Failed to handle command", "type", command.Type, "error", err)
	}
}

// RegisterCommandHandler registers a handler for a specific command type
func (m *MQTTClient) RegisterCommandHandler(commandType string, handler func([]byte) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commandHandlers[commandType] = handler
}

// Event handlers for MQTT client

func (m *MQTTClient) onConnect(client mqtt.Client) {
	m.logger.Info("Connected to MQTT broker")
	m.setConnected(true)
	if err := m.SubscribeToCommands(); err != nil {
		m.logger.Error("Failed to resubscribe to commands", "error", err)
	}
}

func (m *MQTTClient) onConnectionLost(client mqtt.Client, err error) {
	m.logger.Error("Connection lost", "error", err)
	m.setConnected(false)
}

func (m *MQTTClient) onReconnecting(client mqtt.Client, opts *mqtt.ClientOptions) {
	m.logger.Info("Attempting to reconnect to MQTT broker")
}

// Helper methods

func (m *MQTTClient) setConnected(status bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = status
}

// IsConnected returns the connection status
func (m *MQTTClient) IsConnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.connected
}

// Disconnect disconnects from the MQTT broker
func (m *MQTTClient) Disconnect() error {
	if m.client != nil {
		m.client.Disconnect(250)
		m.setConnected(false)
	}
	return nil
}

// Group-related methods

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
	
	// If we're connected, update subscriptions
	if m.isConnected {
		// Subscribe to group content
		contentTopic := fmt.Sprintf("clipman/group/%s/content/+", groupID)
		if token := m.client.Subscribe(contentTopic, 1, m.handleGroupContent); token.Wait() && token.Error() != nil {
			delete(m.groups, groupID)
			return fmt.Errorf("failed to subscribe to group content: %v", token.Error())
		}
		
		// Subscribe to group commands
		commandTopic := fmt.Sprintf("clipman/group/%s/commands/%s", groupID, m.deviceID)
		if token := m.client.Subscribe(commandTopic, 1, m.handleCommand); token.Wait() && token.Error() != nil {
			// Try to unsubscribe from the content topic
			m.client.Unsubscribe(contentTopic)
			delete(m.groups, groupID)
			return fmt.Errorf("failed to subscribe to group commands: %v", token.Error())
		}
		
		// Subscribe to broadcast commands
		broadcastTopic := fmt.Sprintf("clipman/group/%s/commands/broadcast", groupID)
		if token := m.client.Subscribe(broadcastTopic, 1, m.handleCommand); token.Wait() && token.Error() != nil {
			m.logger.Warn("Failed to subscribe to broadcast commands", 
				"group", groupID, "error", token.Error())
			// Non-fatal error, continue
		}
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
	
	// If connected, unsubscribe from group topics
	if m.isConnected {
		contentTopic := fmt.Sprintf("clipman/group/%s/content/+", groupID)
		commandTopic := fmt.Sprintf("clipman/group/%s/commands/%s", groupID, m.deviceID)
		broadcastTopic := fmt.Sprintf("clipman/group/%s/commands/broadcast", groupID)
		
		// Unsubscribe from all topics
		if token := m.client.Unsubscribe(contentTopic, commandTopic, broadcastTopic); token.Wait() && token.Error() != nil {
			m.logger.Warn("Failed to unsubscribe from some group topics", 
				"group", groupID, "error", token.Error())
			// Continue anyway
		}
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
	
	// Future: Implement pattern-based filtering
	// For now, just allow the content
	
	return true
}

// handleGroupContent processes content received from a group
func (m *MQTTClient) handleGroupContent(client mqtt.Client, msg mqtt.Message) {
	// Implementation will decode and process content from groups
	// This stub is here for subscriptions to work
	
	// Topic format is clipman/group/{groupID}/content/{deviceID}
	// Extract the sender device ID from the topic
	
	// Future: Process and possibly store/display the content
	m.logger.Debug("Received group content", "topic", msg.Topic())
}