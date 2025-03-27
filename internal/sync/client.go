// Package sync provides synchronization capabilities between Clipman instances
package sync

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/types"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"
)

// SyncClient defines the interface for clipboard synchronization clients
type SyncClient interface {
	// Connection management
	Connect() error
	Disconnect()
	IsConnected() bool
	
	// Message handling
	Publish(topic string, payload interface{}) error
	Subscribe(topic string, callback types.MessageCallback) error
	Unsubscribe(topic string) error
	
	// Group management
	JoinGroup(groupName string) error
	LeaveGroup(groupName string) error
	ListGroups() ([]string, error)
	
	// Command handling
	RegisterCommandHandler(command string, handler func([]byte) error)
	SendCommand(groupName, command string, payload interface{}) error
	SubscribeToCommands() error
	
	// Content management
	PublishContent(content *types.ClipboardContent) error
	PublishCache(cache *types.CacheMessage) error
	
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
	ID          string             // Unique identifier for the group
	Name        string             // Display name
	Description string             // Group description
	JoinedAt    time.Time          // When this device joined the group
	Members     map[string]string  // Map of DeviceID to DeviceName of group members
}

// Topic constants for MQTT messaging
const (
	// Root topic for all Clipman messages
	TopicClipman = "clipman"
	
	// Group related topics
	TopicGroups     = "groups"
	TopicGroupJoin  = "join"
	TopicGroupLeave = "leave"
	
	// Content related topics
	TopicClipboard = "clipboard"
	
	// Command related topics
	TopicCommand = "command"
)

// CreateClient creates a new SyncClient with the configured mode
func CreateClient(cfg *config.Config, logger *zap.Logger) (SyncClient, error) {
	// TODO: handle sync modes logic here ?
	// discovery will always come from the a fixed remote server
	// centralized will always connect to the same remote server
	// p2p will use a local broker and discover peers via mdns ?
	return NewMQTTClient(cfg, logger)
}

// MQTTClient implements SyncClient interface using MQTT protocol
type MQTTClient struct {
	client          mqtt.Client
	cfg             *config.Config
	logger          *zap.Logger
	deviceID        string
	mu              sync.Mutex
	connected       bool
	commandHandlers map[string]func([]byte) error
	groups          map[string]*SyncGroup
	contentFilter   *ContentFilter
	syncMode        string
}

// NewMQTTClient creates a new MQTT client with synchronization capabilities
func NewMQTTClient(cfg *config.Config, logger *zap.Logger) (*MQTTClient, error) {
	client := &MQTTClient{
		cfg:             cfg,
		logger:          logger,
		deviceID:        cfg.DeviceID,
		commandHandlers: make(map[string]func([]byte) error),
		groups:          make(map[string]*SyncGroup),
		contentFilter:   &ContentFilter{},
		syncMode:        cfg.Sync.Mode,
	}

	// Create MQTT client options
	opts := mqtt.NewClientOptions()
	
	// Configure based on sync mode
	if cfg.Sync.IsModeCentralized() {
		// Centralized mode requires a broker URL
		if cfg.Sync.URL == "" {
			return nil, fmt.Errorf("MQTT broker URL is required for centralized mode")
		}
		
		opts.AddBroker(cfg.Sync.URL)
		
		// Set credentials if provided
		if cfg.Sync.Username != "" {
			opts.SetUsername(cfg.Sync.Username)
			opts.SetPassword(cfg.Sync.Password)
		}
	} else {
		// P2P mode - for now, use a default local broker
		// TODO: Implement P2P discovery mechanism
		opts.AddBroker("tcp://localhost:1883")
	}
	
	// Common options
	opts.SetClientID(fmt.Sprintf("%s-%s", TopicClipman, cfg.DeviceID))
	opts.SetCleanSession(true)
	opts.SetAutoReconnect(true)
	opts.SetConnectionLostHandler(client.onConnectionLost)
	opts.SetOnConnectHandler(client.onConnect)
	
	// Create client
	client.client = mqtt.NewClient(opts)
	
	return client, nil
}

// Connect establishes a connection to the MQTT broker
func (m *MQTTClient) Connect() error {
	if token := m.client.Connect(); token.Wait() && token.Error() != nil {
		return token.Error()
	}
	
	m.setConnected(true)
	m.logger.Info("Connected to MQTT broker")
	
	// Auto-join default group if configured
	if m.cfg.Sync.DefaultGroup != "" && m.cfg.Sync.AutoJoinGroups {
		if err := m.JoinGroup(m.cfg.Sync.DefaultGroup); err != nil {
			m.logger.Error("Failed to auto-join default group", zap.Error(err))
		}
	}
	
	return nil
}

// Disconnect closes the connection to the MQTT broker
func (m *MQTTClient) Disconnect() {
	if m.client != nil && m.IsConnected() {
		m.client.Disconnect(250)
		m.setConnected(false)
	}
	m.logger.Info("Disconnected from MQTT broker")
}

// IsConnected returns true if connected to the broker
func (m *MQTTClient) IsConnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.client == nil {
		return false
	}
	return m.connected && m.client.IsConnected()
}

// setConnected safely updates the connection status
func (m *MQTTClient) setConnected(status bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.connected = status
}

// JoinGroup subscribes to a group's topics
func (m *MQTTClient) JoinGroup(groupName string) error {
	if !m.IsConnected() {
		if err := m.Connect(); err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Check if already joined
	if _, exists := m.groups[groupName]; exists {
		return nil // Already joined
	}
	
	// Create new group
	m.groups[groupName] = &SyncGroup{
		Name:    groupName,
		Members: make(map[string]string),
		JoinedAt: time.Now(),
	}
	
	// Subscribe to group's clipboard topic
	clipboardTopic := fmt.Sprintf("%s/%s/%s/#", TopicClipman, TopicGroups, groupName)
	if err := m.subscribeUnsafe(clipboardTopic, m.handleClipboardMessage); err != nil {
		delete(m.groups, groupName)
		return fmt.Errorf("failed to subscribe to group clipboard: %w", err)
	}
	
	// Subscribe to group's command topic
	commandTopic := fmt.Sprintf("%s/%s/%s/%s/#", TopicClipman, TopicGroups, groupName, TopicCommand)
	if err := m.subscribeUnsafe(commandTopic, m.handleCommandMessage); err != nil {
		delete(m.groups, groupName)
		return fmt.Errorf("failed to subscribe to group commands: %w", err)
	}
	
	// Send join message
	joinTopic := fmt.Sprintf("%s/%s/%s/%s", TopicClipman, TopicGroups, groupName, TopicGroupJoin)
	joinPayload := struct {
		DeviceID   string    `json:"device_id"`
		DeviceName string    `json:"device_name"`
		Timestamp  time.Time `json:"timestamp"`
	}{
		DeviceID:   m.deviceID,
		DeviceName: m.cfg.DeviceName,
		Timestamp:  time.Now(),
	}
	
	if err := m.publishUnsafe(joinTopic, joinPayload); err != nil {
		return fmt.Errorf("failed to publish join message: %w", err)
	}
	
	m.logger.Info("Joined sync group", zap.String("group", groupName))
	return nil
}

// LeaveGroup unsubscribes from a group's topics
func (m *MQTTClient) LeaveGroup(groupName string) error {
	if !m.IsConnected() {
		return fmt.Errorf("not connected to MQTT broker")
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	// Check if joined
	if _, exists := m.groups[groupName]; !exists {
		return nil // Not joined
	}
	
	// Send leave message
	leaveTopic := fmt.Sprintf("%s/%s/%s/%s", TopicClipman, TopicGroups, groupName, TopicGroupLeave)
	leavePayload := struct {
		DeviceID   string    `json:"device_id"`
		DeviceName string    `json:"device_name"`
		Timestamp  time.Time `json:"timestamp"`
	}{
		DeviceID:   m.deviceID,
		DeviceName: m.cfg.DeviceName,
		Timestamp:  time.Now(),
	}
	
	_ = m.publishUnsafe(leaveTopic, leavePayload) // Best effort
	
	// Unsubscribe from group's clipboard topic
	clipboardTopic := fmt.Sprintf("%s/%s/%s/#", TopicClipman, TopicGroups, groupName)
	_ = m.unsubscribeUnsafe(clipboardTopic) // Best effort
	
	// Unsubscribe from group's command topic
	commandTopic := fmt.Sprintf("%s/%s/%s/%s/#", TopicClipman, TopicGroups, groupName, TopicCommand)
	_ = m.unsubscribeUnsafe(commandTopic) // Best effort
	
	// Remove group
	delete(m.groups, groupName)
	
	m.logger.Info("Left sync group", zap.String("group", groupName))
	return nil
}

// ListGroups returns a list of groups the client has joined
func (m *MQTTClient) ListGroups() ([]string, error) {
	if !m.IsConnected() {
		return nil, fmt.Errorf("not connected to MQTT broker")
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	groups := make([]string, 0, len(m.groups))
	for name := range m.groups {
		groups = append(groups, name)
	}
	
	return groups, nil
}

// Publish publishes a message to the specified topic
func (m *MQTTClient) Publish(topic string, payload interface{}) error {
	if !m.IsConnected() {
		if err := m.Connect(); err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	return m.publishUnsafe(topic, payload)
}

// publishUnsafe publishes a message without checking connection state or locking
// Caller must hold the mutex
func (m *MQTTClient) publishUnsafe(topic string, payload interface{}) error {
	// Convert payload to JSON
	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}
	
	// Publish message
	token := m.client.Publish(topic, 0, false, jsonPayload)
	token.Wait()
	
	if err := token.Error(); err != nil {
		m.logger.Error("Failed to publish message", 
			zap.String("topic", topic), 
			zap.Error(err))
		return err
	}
	
	m.logger.Debug("Published message", 
		zap.String("topic", topic), 
		zap.Int("payload_size", len(jsonPayload)))
	
	return nil
}

// Subscribe subscribes to the specified topic
func (m *MQTTClient) Subscribe(topic string, callback types.MessageCallback) error {
	if !m.IsConnected() {
		if err := m.Connect(); err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	return m.subscribeUnsafe(topic, callback)
}

// subscribeUnsafe subscribes to a topic without checking connection state or locking
// Caller must hold the mutex
func (m *MQTTClient) subscribeUnsafe(topic string, callback types.MessageCallback) error {
	// Create wrapper function to convert MQTT message to our message type
	wrapper := func(client mqtt.Client, msg mqtt.Message) {
		message := types.Message{
			Topic:   msg.Topic(),
			Payload: msg.Payload(),
		}
		callback(message)
	}
	
	// Subscribe to topic
	token := m.client.Subscribe(topic, 0, wrapper)
	token.Wait()
	
	if err := token.Error(); err != nil {
		m.logger.Error("Failed to subscribe", 
			zap.String("topic", topic), 
			zap.Error(err))
		return err
	}
	
	m.logger.Debug("Subscribed to topic", zap.String("topic", topic))
	return nil
}

// Unsubscribe unsubscribes from the specified topic
func (m *MQTTClient) Unsubscribe(topic string) error {
	if !m.IsConnected() {
		return fmt.Errorf("not connected to MQTT broker")
	}
	
	m.mu.Lock()
	defer m.mu.Unlock()
	
	return m.unsubscribeUnsafe(topic)
}

// unsubscribeUnsafe unsubscribes from a topic without checking connection state or locking
// Caller must hold the mutex
func (m *MQTTClient) unsubscribeUnsafe(topic string) error {
	// Unsubscribe from topic
	token := m.client.Unsubscribe(topic)
	token.Wait()
	
	if err := token.Error(); err != nil {
		m.logger.Error("Failed to unsubscribe", 
			zap.String("topic", topic), 
			zap.Error(err))
		return err
	}
	
	m.logger.Debug("Unsubscribed from topic", zap.String("topic", topic))
	return nil
}

// RegisterCommandHandler registers a handler for a specific command
func (m *MQTTClient) RegisterCommandHandler(command string, handler func([]byte) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	m.commandHandlers[command] = handler
	m.logger.Debug("Registered command handler", zap.String("command", command))
}

// SendCommand sends a command to a specific group
func (m *MQTTClient) SendCommand(groupName, command string, payload interface{}) error {
	if !m.IsConnected() {
		if err := m.Connect(); err != nil {
			return fmt.Errorf("failed to connect: %w", err)
		}
	}
	
	commandTopic := fmt.Sprintf("%s/%s/%s/%s/%s", 
		TopicClipman, TopicGroups, groupName, TopicCommand, command)
	
	return m.Publish(commandTopic, payload)
}

// onConnect is called when the client connects to the broker
func (m *MQTTClient) onConnect(client mqtt.Client) {
	m.setConnected(true)
	m.logger.Info("Connected to MQTT broker")
	
	// Restore subscriptions
	m.mu.Lock()
	defer m.mu.Unlock()
	
	for groupName := range m.groups {
		// Resubscribe to group topics
		clipboardTopic := fmt.Sprintf("%s/%s/%s/#", TopicClipman, TopicGroups, groupName)
		if err := m.subscribeUnsafe(clipboardTopic, m.handleClipboardMessage); err != nil {
			m.logger.Error("Failed to resubscribe to group clipboard", 
				zap.String("group", groupName),
				zap.Error(err))
		}
		
		commandTopic := fmt.Sprintf("%s/%s/%s/%s/#", TopicClipman, TopicGroups, groupName, TopicCommand)
		if err := m.subscribeUnsafe(commandTopic, m.handleCommandMessage); err != nil {
			m.logger.Error("Failed to resubscribe to group commands", 
				zap.String("group", groupName),
				zap.Error(err))
		}
	}
}

// onConnectionLost is called when the client loses connection to the broker
func (m *MQTTClient) onConnectionLost(client mqtt.Client, err error) {
	m.setConnected(false)
	m.logger.Error("Lost connection to MQTT broker", zap.Error(err))
}

// handleClipboardMessage processes incoming clipboard messages
func (m *MQTTClient) handleClipboardMessage(msg types.Message) {
	m.logger.Debug("Received clipboard message", zap.String("topic", msg.Topic))
	
	// TODO: Process clipboard content
	// - Parse the JSON payload
	// - Apply content filtering rules
	// - Update local clipboard if appropriate
}

// handleCommandMessage processes incoming command messages
func (m *MQTTClient) handleCommandMessage(msg types.Message) {
	m.logger.Debug("Received command message", zap.String("topic", msg.Topic))
	
	// Extract command from topic
	// Expected format: clipman/groups/{group}/command/{command}
	parts := splitTopic(msg.Topic)
	if len(parts) < 5 {
		m.logger.Error("Invalid command topic format", zap.String("topic", msg.Topic))
		return
	}
	
	command := parts[4]
	
	// Find handler for this command
	m.mu.Lock()
	handler, exists := m.commandHandlers[command]
	m.mu.Unlock()
	
	if !exists {
		m.logger.Error("No handler for command", zap.String("command", command))
		return
	}
	
	// Execute handler
	if err := handler(msg.Payload); err != nil {
		m.logger.Error("Command handler failed", 
			zap.String("command", command),
			zap.Error(err))
	}
}

// Helper function to split a topic string into parts
func splitTopic(topic string) []string {
	var parts []string
	start := 0
	for i := 0; i < len(topic); i++ {
		if topic[i] == '/' {
			if i > start {
				parts = append(parts, topic[start:i])
			}
			start = i + 1
		}
	}
	if start < len(topic) {
		parts = append(parts, topic[start:])
	}
	return parts
}

// PublishContent publishes clipboard content to subscribed devices
func (m *MQTTClient) PublishContent(content *types.ClipboardContent) error {
	if !m.shouldPublishContent(content) {
		m.logger.Debug("Content filtered, not publishing", 
			zap.String("content_type", string(content.Type)),
			zap.Int("content_size", len(content.Data)))
		return nil
	}

	// Build the content message
	payload := struct {
		DeviceID   string                  `json:"device_id"`
		DeviceName string                  `json:"device_name"`
		Timestamp  time.Time               `json:"timestamp"`
		Content    *types.ClipboardContent `json:"content"`
	}{
		DeviceID:   m.deviceID,
		DeviceName: m.cfg.DeviceName,
		Timestamp:  time.Now(),
		Content:    content,
	}

	// For each group, publish to the clipboard topic
	m.mu.Lock()
	activeGroups := make([]string, 0, len(m.groups))
	for groupName := range m.groups {
		activeGroups = append(activeGroups, groupName)
	}
	m.mu.Unlock()

	if len(activeGroups) == 0 {
		// If not in any groups, simply return without error
		return nil
	}

	// Publish to each group
	var lastErr error
	for _, groupName := range activeGroups {
		topic := fmt.Sprintf("%s/%s/%s/%s/%s", 
			TopicClipman, TopicGroups, groupName, TopicClipboard, m.deviceID)
		
		if err := m.Publish(topic, payload); err != nil {
			m.logger.Error("Failed to publish clipboard content", 
				zap.String("group", groupName),
				zap.Error(err))
			lastErr = err
		}
	}

	return lastErr
}

// PublishCache broadcasts a cache update message to synced devices
func (m *MQTTClient) PublishCache(cache *types.CacheMessage) error {
	// Build the cache message
	payload := struct {
		DeviceID   string             `json:"device_id"`
		DeviceName string             `json:"device_name"`
		Timestamp  time.Time          `json:"timestamp"`
		Cache      *types.CacheMessage `json:"cache"`
	}{
		DeviceID:   m.deviceID,
		DeviceName: m.cfg.DeviceName,
		Timestamp:  time.Now(),
		Cache:      cache,
	}

	// For each group, publish to the cache topic
	m.mu.Lock()
	activeGroups := make([]string, 0, len(m.groups))
	for groupName := range m.groups {
		activeGroups = append(activeGroups, groupName)
	}
	m.mu.Unlock()

	if len(activeGroups) == 0 {
		// If not in any groups, simply return without error
		return nil
	}

	// Publish to each group
	var lastErr error
	for _, groupName := range activeGroups {
		topic := fmt.Sprintf("%s/%s/%s/cache/%s", 
			TopicClipman, TopicGroups, groupName, m.deviceID)
		
		if err := m.Publish(topic, payload); err != nil {
			m.logger.Error("Failed to publish cache update", 
				zap.String("group", groupName),
				zap.Error(err))
			lastErr = err
		}
	}

	return lastErr
}

// shouldPublishContent checks if content should be published based on filters
func (m *MQTTClient) shouldPublishContent(content *types.ClipboardContent) bool {
	m.mu.Lock()
	filter := m.contentFilter
	m.mu.Unlock()

	// If no filter set, allow all
	if filter == nil {
		return true
	}

	// Check content type
	if len(filter.AllowedTypes) > 0 {
		allowed := false
		for _, allowedType := range filter.AllowedTypes {
			if string(allowedType) == string(content.Type) {
				allowed = true
				break
			}
		}
		if !allowed {
			return false
		}
	}

	// Check excluded types
	if len(filter.ExcludedTypes) > 0 {
		for _, excludedType := range filter.ExcludedTypes {
			if string(excludedType) == string(content.Type) {
				return false
			}
		}
	}

	// Check maximum size
	if filter.MaxSize > 0 && int64(len(content.Data)) > filter.MaxSize {
		return false
	}

	// Add pattern matching here if needed

	return true
}

// SetContentFilter sets the filtering criteria for content synchronization
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

// SubscribeToCommands subscribes to command messages
func (m *MQTTClient) SubscribeToCommands() error {
	if !m.IsConnected() {
		return fmt.Errorf("not connected to MQTT broker")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// For each group, subscribe to command topics
	for groupName := range m.groups {
		// Subscribe to group commands
		commandTopic := fmt.Sprintf("%s/%s/%s/%s/#", TopicClipman, TopicGroups, groupName, TopicCommand)
		if err := m.subscribeUnsafe(commandTopic, m.handleCommandMessage); err != nil {
			m.logger.Error("Failed to subscribe to group commands", 
				zap.String("group", groupName),
				zap.Error(err))
		}
	}

	// For backward compatibility, also subscribe to legacy command topics
	legacyTopic := fmt.Sprintf("clipman/%s/commands", m.deviceID)
	wrapper := func(msg types.Message) {
		if msg.Topic == legacyTopic {
			m.handleLegacyCommand(msg.Payload)
		}
	}
	
	if err := m.subscribeUnsafe(legacyTopic, wrapper); err != nil {
		m.logger.Warn("Failed to subscribe to legacy commands", zap.Error(err))
	}
	
	return nil
}

// handleLegacyCommand handles commands received in the legacy format
func (m *MQTTClient) handleLegacyCommand(payload []byte) {
	// Try to determine the command type from the payload
	var cmd struct {
		Type string `json:"type"`
		Data json.RawMessage `json:"data"`
	}
	
	if err := json.Unmarshal(payload, &cmd); err != nil {
		m.logger.Error("Failed to unmarshal legacy command", zap.Error(err))
		return
	}
	
	// Find the handler for this command type
	m.mu.Lock()
	handler, exists := m.commandHandlers[cmd.Type]
	m.mu.Unlock()
	
	if !exists {
		m.logger.Error("No handler for legacy command type", zap.String("type", cmd.Type))
		return
	}
	
	// Execute the handler with the command data
	if err := handler([]byte(cmd.Data)); err != nil {
		m.logger.Error("Legacy command handler failed", 
			zap.String("type", cmd.Type), 
			zap.Error(err))
	}
} 