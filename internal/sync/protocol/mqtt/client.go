// Package mqtt provides an MQTT-based implementation of the sync protocol
// This file implements the MQTT client for the sync protocol
package mqtt

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/sync"
	"github.com/berrythewa/clipman-daemon/internal/sync/protocol"
	"github.com/berrythewa/clipman-daemon/internal/types"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"go.uber.org/zap"
)

// MQTTOptions contains MQTT-specific configuration options
type MQTTOptions struct {
	protocol.ProtocolOptions
	Broker       string // Broker URL
	ClientID     string // Client ID
	Username     string // Username for authentication
	Password     string // Password for authentication
	UseTLS       bool   // Whether to use TLS
	QoS          byte   // Default QoS level
	CleanSession bool   // Whether to use a clean session
	KeepAlive    int    // Keep alive interval in seconds
	Logger       *zap.Logger // Logger for MQTT client
}

// DefaultMQTTOptions returns the default MQTT options
func DefaultMQTTOptions() *MQTTOptions {
	return &MQTTOptions{
		ProtocolOptions: protocol.ProtocolOptions{
			ReconnectDelay:    5 * time.Second,
			ReconnectMaxRetry: 12, // Try for 1 minute
		},
		Broker:       "tcp://localhost:1883",
		ClientID:     "",  // Will be generated if empty
		QoS:          QoSAtLeastOnce,
		CleanSession: true,
		KeepAlive:    60,
		Logger:       zap.NewNop(), // Default to no-op logger
	}
}

// MQTTClient implements the protocol.Client interface for MQTT
type MQTTClient struct {
	opts        *MQTTOptions
	client      mqtt.Client
	isConnected bool
	handlers    []protocol.MessageHandler
	groups      []string
	mutex       sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	logger      *zap.Logger
}

// NewMQTTClient creates a new MQTT client
func NewMQTTClient(opts *MQTTOptions) (*MQTTClient, error) {
	if opts == nil {
		opts = DefaultMQTTOptions()
	}
	
	// Use default logger if not provided
	logger := opts.Logger
	if logger == nil {
		logger = zap.NewNop()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	client := &MQTTClient{
		opts:     opts,
		handlers: make([]protocol.MessageHandler, 0),
		groups:   make([]string, 0),
		ctx:      ctx,
		cancel:   cancel,
		logger:   logger.With(zap.String("component", "mqtt_client")),
	}
	
	return client, nil
}

// SetLogger sets the logger for the client
func (c *MQTTClient) SetLogger(logger *zap.Logger) {
	if logger != nil {
		c.mutex.Lock()
		c.logger = logger.With(zap.String("component", "mqtt_client"))
		c.mutex.Unlock()
	}
}

// Connect connects to the MQTT broker
func (c *MQTTClient) Connect() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	if c.isConnected {
		return nil
	}
	
	c.logger.Info("Connecting to MQTT broker", zap.String("broker", c.opts.Broker))
	
	// Create MQTT client options
	mqttOpts := mqtt.NewClientOptions()
	mqttOpts.AddBroker(c.opts.Broker)
	
	// Set client ID if provided, otherwise let the broker generate one
	if c.opts.ClientID != "" {
		mqttOpts.SetClientID(c.opts.ClientID)
	}
	
	// Set credentials if provided
	if c.opts.Username != "" {
		mqttOpts.SetUsername(c.opts.Username)
		mqttOpts.SetPassword(c.opts.Password)
	}
	
	// Set other options
	mqttOpts.SetCleanSession(c.opts.CleanSession)
	mqttOpts.SetKeepAlive(time.Duration(c.opts.KeepAlive) * time.Second)
	mqttOpts.SetAutoReconnect(true)
	mqttOpts.SetMaxReconnectInterval(c.opts.ReconnectDelay)
	
	// Set connect handler
	mqttOpts.SetOnConnectHandler(func(client mqtt.Client) {
		c.logger.Info("Connected to MQTT broker", zap.String("broker", c.opts.Broker))
		c.isConnected = true
		
		// Re-subscribe to all groups
		c.mutex.RLock()
		groups := make([]string, len(c.groups))
		copy(groups, c.groups)
		c.mutex.RUnlock()
		
		for _, group := range groups {
			if err := c.subscribeToGroup(group); err != nil {
				c.logger.Error("Failed to re-subscribe to group", 
					zap.String("group", group), 
					zap.Error(err))
			}
		}
	})
	
	// Set connection lost handler
	mqttOpts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		c.logger.Warn("Lost connection to MQTT broker", 
			zap.String("broker", c.opts.Broker),
			zap.Error(err))
		c.isConnected = false
	})
	
	// Create and connect to the broker
	c.client = mqtt.NewClient(mqttOpts)
	token := c.client.Connect()
	if token.Wait() && token.Error() != nil {
		c.logger.Error("Failed to connect to MQTT broker", 
			zap.String("broker", c.opts.Broker),
			zap.Error(token.Error()))
		return fmt.Errorf("failed to connect to MQTT broker: %w", token.Error())
	}
	
	return nil
}

// Disconnect disconnects from the MQTT broker
func (c *MQTTClient) Disconnect() error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	if !c.isConnected {
		return nil
	}
	
	c.logger.Info("Disconnecting from MQTT broker", zap.String("broker", c.opts.Broker))
	
	c.cancel()
	c.client.Disconnect(250) // Wait 250ms for in-flight messages
	c.isConnected = false
	
	c.logger.Info("Disconnected from MQTT broker", zap.String("broker", c.opts.Broker))
	
	return nil
}

// IsConnected returns whether the client is connected to the broker
func (c *MQTTClient) IsConnected() bool {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	return c.isConnected
}

// JoinGroup joins a synchronization group
func (c *MQTTClient) JoinGroup(group string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	// Check if already joined
	for _, g := range c.groups {
		if g == group {
			return nil
		}
	}
	
	c.logger.Info("Joining group", zap.String("group", group))
	
	// Add to internal list
	c.groups = append(c.groups, group)
	
	// Subscribe if connected
	if c.isConnected {
		return c.subscribeToGroup(group)
	}
	
	return nil
}

// LeaveGroup leaves a synchronization group
func (c *MQTTClient) LeaveGroup(group string) error {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	// Find the group
	idx := -1
	for i, g := range c.groups {
		if g == group {
			idx = i
			break
		}
	}
	
	if idx == -1 {
		return nil // Not in group
	}
	
	c.logger.Info("Leaving group", zap.String("group", group))
	
	// Remove from internal list
	c.groups = append(c.groups[:idx], c.groups[idx+1:]...)
	
	// Unsubscribe if connected
	if c.isConnected {
		return c.unsubscribeFromGroup(group)
	}
	
	return nil
}

// ListGroups returns the list of joined groups
func (c *MQTTClient) ListGroups() []string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()
	
	groups := make([]string, len(c.groups))
	copy(groups, c.groups)
	
	return groups
}

// Send sends a message
func (c *MQTTClient) Send(msg sync.Message) error {
	c.mutex.RLock()
	connected := c.isConnected
	c.mutex.RUnlock()
	
	if !connected {
		return fmt.Errorf("not connected to MQTT broker")
	}
	
	c.logger.Debug("Sending message", 
		zap.String("type", msg.Type()),
		zap.String("group", msg.Group()))
	
	// Convert to MQTT message if not already
	mqttMsg, ok := msg.(*MQTTMessage)
	if !ok {
		// Try to convert
		data, err := msg.ToJSON()
		if err != nil {
			c.logger.Error("Failed to convert message to MQTT format", zap.Error(err))
			return fmt.Errorf("failed to convert message to MQTT: %w", err)
		}
		
		// Deserialize into MQTT message
		mqttMsg = &MQTTMessage{}
		if err := mqttMsg.FromJSON(data); err != nil {
			c.logger.Error("Failed to parse converted message", zap.Error(err))
			return fmt.Errorf("failed to convert message to MQTT: %w", err)
		}
	}
	
	// Get topic
	topic := mqttMsg.GetTopic()
	
	// Convert payload to bytes
	payload := mqttMsg.Payload()
	
	c.logger.Debug("Publishing MQTT message", 
		zap.String("topic", topic),
		zap.Int("payload_size", len(payload)),
		zap.Uint8("qos", mqttMsg.QoS))
	
	// Publish message
	token := c.client.Publish(topic, mqttMsg.QoS, mqttMsg.Retain, payload)
	if token.Wait() && token.Error() != nil {
		c.logger.Error("Failed to publish message", 
			zap.String("topic", topic),
			zap.Error(token.Error()))
		return fmt.Errorf("failed to publish message: %w", token.Error())
	}
	
	c.logger.Debug("Message published successfully", zap.String("topic", topic))
	
	return nil
}

// AddHandler adds a message handler
func (c *MQTTClient) AddHandler(handler protocol.MessageHandler) {
	c.mutex.Lock()
	defer c.mutex.Unlock()
	
	c.handlers = append(c.handlers, handler)
	c.logger.Debug("Message handler added", 
		zap.Int("total_handlers", len(c.handlers)))
}

// subscribeToGroup subscribes to all topics for a group
func (c *MQTTClient) subscribeToGroup(group string) error {
	// Create topic filters for content and control messages
	contentTopic := fmt.Sprintf("%s/%s/%s/#", TopicPrefix, group, TopicContentSuffix)
	controlTopic := fmt.Sprintf("%s/%s/%s/#", TopicPrefix, group, TopicControlSuffix)
	
	c.logger.Debug("Subscribing to group topics", 
		zap.String("group", group),
		zap.String("content_topic", contentTopic),
		zap.String("control_topic", controlTopic))
	
	// Subscribe to content topic
	token := c.client.Subscribe(contentTopic, c.opts.QoS, c.messageHandler)
	if token.Wait() && token.Error() != nil {
		c.logger.Error("Failed to subscribe to content topic", 
			zap.String("topic", contentTopic),
			zap.Error(token.Error()))
		return fmt.Errorf("failed to subscribe to content topic: %w", token.Error())
	}
	
	// Subscribe to control topic
	token = c.client.Subscribe(controlTopic, c.opts.QoS, c.messageHandler)
	if token.Wait() && token.Error() != nil {
		c.logger.Error("Failed to subscribe to control topic", 
			zap.String("topic", controlTopic),
			zap.Error(token.Error()))
		return fmt.Errorf("failed to subscribe to control topic: %w", token.Error())
	}
	
	c.logger.Info("Subscribed to group topics", zap.String("group", group))
	
	return nil
}

// unsubscribeFromGroup unsubscribes from all topics for a group
func (c *MQTTClient) unsubscribeFromGroup(group string) error {
	// Create topic filters for content and control messages
	contentTopic := fmt.Sprintf("%s/%s/%s/#", TopicPrefix, group, TopicContentSuffix)
	controlTopic := fmt.Sprintf("%s/%s/%s/#", TopicPrefix, group, TopicControlSuffix)
	
	c.logger.Debug("Unsubscribing from group topics", 
		zap.String("group", group),
		zap.String("content_topic", contentTopic),
		zap.String("control_topic", controlTopic))
	
	// Unsubscribe from content topic
	token := c.client.Unsubscribe(contentTopic)
	if token.Wait() && token.Error() != nil {
		c.logger.Error("Failed to unsubscribe from content topic", 
			zap.String("topic", contentTopic),
			zap.Error(token.Error()))
		return fmt.Errorf("failed to unsubscribe from content topic: %w", token.Error())
	}
	
	// Unsubscribe from control topic
	token = c.client.Unsubscribe(controlTopic)
	if token.Wait() && token.Error() != nil {
		c.logger.Error("Failed to unsubscribe from control topic", 
			zap.String("topic", controlTopic),
			zap.Error(token.Error()))
		return fmt.Errorf("failed to unsubscribe from control topic: %w", token.Error())
	}
	
	c.logger.Info("Unsubscribed from group topics", zap.String("group", group))
	
	return nil
}

// messageHandler handles incoming MQTT messages
func (c *MQTTClient) messageHandler(client mqtt.Client, mqttMsg mqtt.Message) {
	// Parse the message
	topic := mqttMsg.Topic()
	payload := mqttMsg.Payload()
	
	c.logger.Debug("Received MQTT message", 
		zap.String("topic", topic),
		zap.Int("payload_size", len(payload)))
	
	// Convert to sync.Message
	msg, err := ParseMQTTMessage(topic, payload)
	if err != nil {
		c.logger.Error("Failed to parse MQTT message", 
			zap.String("topic", topic),
			zap.Error(err))
		return
	}
	
	c.logger.Debug("Parsed MQTT message",
		zap.String("type", msg.Type()),
		zap.String("source", msg.Source()),
		zap.String("group", msg.Group()))
	
	// Get handlers
	c.mutex.RLock()
	handlers := make([]protocol.MessageHandler, len(c.handlers))
	copy(handlers, c.handlers)
	c.mutex.RUnlock()
	
	// Call all handlers
	for _, handler := range handlers {
		if handler != nil {
			go func(h protocol.MessageHandler, m sync.Message) {
				if err := h(m); err != nil {
					c.logger.Error("Handler error processing message",
						zap.String("type", m.Type()),
						zap.String("group", m.Group()),
						zap.Error(err))
				}
			}(handler, msg)
		}
	}
}

// Factory is the MQTT protocol factory
type Factory struct{}

// NewFactory creates a new MQTT protocol factory
func NewFactory() *Factory {
	return &Factory{}
}

// NewClient creates a new MQTT client
func (f *Factory) NewClient(options interface{}) (protocol.Client, error) {
	// Convert options
	mqttOpts, ok := options.(*MQTTOptions)
	if !ok {
		// Check if we received a generic ProtocolOptions
		if genericOpts, ok := options.(*protocol.ProtocolOptions); ok {
			mqttOpts = DefaultMQTTOptions()
			mqttOpts.ProtocolOptions = *genericOpts
		} else {
			return nil, fmt.Errorf("invalid options type, expected *MQTTOptions but got %T", options)
		}
	}
	
	// Create client
	return NewMQTTClient(mqttOpts)
}

// CreateContentMessage creates a new MQTT content message
func (f *Factory) CreateContentMessage(content *types.ClipboardContent) (sync.Message, error) {
	if content == nil {
		return nil, fmt.Errorf("content cannot be nil")
	}
	
	return NewMQTTContentMessage(content)
} 