package broker

import (
	"encoding/json"
	"fmt"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/berrythewa/clipman-daemon/internal/clipboard"
	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/pkg/utils"
)

type MQTTClient struct {
	client     mqtt.Client
	config     *config.Config
	logger     *utils.Logger
	deviceID   string
}

func NewMQTTClient(cfg *config.Config, logger *utils.Logger) (*MQTTClient, error) {
	opts := mqtt.NewClientOptions().
		AddBroker(cfg.BrokerURL).
		SetClientID(cfg.DeviceID).
		SetUsername(cfg.BrokerUsername).
		SetPassword(cfg.BrokerPassword).
		SetAutoReconnect(true).
		SetOnConnectHandler(onConnect).
		SetConnectionLostHandler(onConnectionLost)

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		return nil, fmt.Errorf("failed to connect to MQTT broker: %v", token.Error())
	}

	return &MQTTClient{
		client:   client,
		config:   cfg,
		logger:   logger,
		deviceID: cfg.DeviceID,
	}, nil
}

func (m *MQTTClient) PublishContent(content *clipboard.ClipboardContent) error {
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

func (m *MQTTClient) SubscribeToCommands() error {
	topic := fmt.Sprintf("clipman/%s/commands", m.deviceID)
	token := m.client.Subscribe(topic, 1, func(client mqtt.Client, msg mqtt.Message) {
		m.handleCommand(msg.Payload())
	})

	if token.Wait() && token.Error() != nil {
		return fmt.Errorf("failed to subscribe to commands: %v", token.Error())
	}

	return nil
}

func (m *MQTTClient) handleCommand(payload []byte) {
	// Implement command handling logic here
	// For example, you might receive commands to:
	// - Update configuration
	// - Request current clipboard content
	// - Pause/resume monitoring
	m.logger.Info("Received command", "payload", string(payload))
}

func onConnect(client mqtt.Client) {
	fmt.Println("Connected to MQTT broker")
}

func onConnectionLost(client mqtt.Client, err error) {
	fmt.Printf("Connection lost: %v\n", err)
}