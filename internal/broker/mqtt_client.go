package broker

import (
	"encoding/json"
	"fmt"
	"sync"
	// "time"
	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/berrythewa/clipman-daemon/pkg/utils"
	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// MQTTClientInterface defines the methods for the MQTTClient.
type MQTTClientInterface interface {
	PublishContent(content *types.ClipboardContent) error
	SubscribeToCommands() error
	RegisterCommandHandler(commandType string, handler func([]byte) error)
	IsConnected() bool
	Disconnect()
}

type MQTTClient struct {
	client          mqtt.Client
	config          *config.Config
	logger          *utils.Logger
	deviceID        string
	mu              sync.Mutex
	isConnected     bool
	commandHandlers map[string]func([]byte) error
}

func NewMQTTClient(cfg *config.Config, logger *utils.Logger) (*MQTTClient, error) {
	client := &MQTTClient{
		config:          cfg,
		logger:          logger,
		deviceID:        cfg.DeviceID,
		commandHandlers: make(map[string]func([]byte) error),
	}

	opts := mqtt.NewClientOptions().
		AddBroker(cfg.BrokerURL).
		SetClientID(cfg.DeviceID).
		SetUsername(cfg.BrokerUsername).
		SetPassword(cfg.BrokerPassword).
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

func (m *MQTTClient) connect() error {
	token := m.client.Connect()
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to connect to MQTT broker: %v", token.Error())
	}
	m.setConnected(true)
	return nil
}

func (m *MQTTClient) PublishContent(content *types.ClipboardContent) error {
	if m == nil || m.client == nil {
		return nil // Skip publishing if no MQTT client
	}

	if !m.isConnected {
		return fmt.Errorf("not connected to MQTT broker")
	}

	topic := fmt.Sprintf("clipman/%s/content", m.deviceID)
	payload, err := json.Marshal(content)
	if err != nil {
		return fmt.Errorf("failed to marshal clipboard content: %v", err)
	}

	token := m.client.Publish(topic, 1, false, payload)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish content: %v", token.Error())
	}

	return nil
}

// func (m *Monitor) publishContent(content *types.ClipboardContent) error {
//     if m.mqttClient == nil {
//         return nil
//     }

//     // For local paths/URLs, load actual content before publishing
//     switch content.Type {
//     case types.TypeImage, types.TypeFile:
//         processedContent, err := m.loadContentForPublishing(content)
//         if err != nil {
//             m.logger.Error("Failed to load content for publishing", "error", err)
//             return err
//         }
//         return m.mqttClient.PublishContent(processedContent)
//     default:
//         return m.mqttClient.PublishContent(content)
//     }
// }

func (c *MQTTClient) PublishCache(cache *types.CacheMessage) error {
	topic := fmt.Sprintf("clipman/cache/%s", cache.DeviceID)

	payload, err := json.Marshal(cache)
	if err != nil {
		return fmt.Errorf("failed to marshal cache: %v", err)
	}

	if token := c.client.Publish(topic, 1, false, payload); token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to publish cache: %v", token.Error())
	}

	return nil
}

func (m *MQTTClient) SubscribeToCommands() error {
	if !m.isConnected {
		return fmt.Errorf("not connected to MQTT broker")
	}

	topic := fmt.Sprintf("clipman/%s/commands", m.deviceID)
	token := m.client.Subscribe(topic, 1, m.handleCommand)
	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to subscribe to commands: %v", token.Error())
	}

	return nil
}

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

func (m *MQTTClient) RegisterCommandHandler(commandType string, handler func([]byte) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.commandHandlers[commandType] = handler
}

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

func (m *MQTTClient) setConnected(status bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isConnected = status
}

func (m *MQTTClient) IsConnected() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isConnected
}

func (m *MQTTClient) Disconnect() {
	m.client.Disconnect(250)
	m.setConnected(false)
}
