package clipboard

import (
	"time"

	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/broker"
	"github.com/berrythewa/clipman-daemon/internal/storage"
	"github.com/berrythewa/clipman-daemon/pkg/utils"
)

type Monitor struct {
	config     *config.Config
	mqttClient *broker.MQTTClient
	logger     *utils.Logger
	clipboard  Clipboard
	storage    *storage.BoltStorage
	lastContent *ClipboardContent
}

func NewMonitor(cfg *config.Config, mqttClient *broker.MQTTClient, logger *utils.Logger, storage *storage.BoltStorage) *Monitor {
	return &Monitor{
		config:     cfg,
		mqttClient: mqttClient,
		logger:     logger,
		clipboard:  NewClipboard(),
		storage:    storage,
	}
}

func (m *Monitor) Start() error {
	m.logger.Info("Starting clipboard monitor")
	
	// Try to load the last content from storage
	lastContent, err := m.storage.GetLatestContent()
	if err != nil {
		m.logger.Error("Failed to get latest content from storage", "error", err)
	} else if lastContent != nil {
		m.lastContent = lastContent
	}

	go m.monitorClipboard()
	return nil
}

func (m *Monitor) monitorClipboard() {
	for {
		content, err := m.clipboard.Read()
		if err != nil {
			m.logger.Error("Error reading clipboard", "error", err)
			time.Sleep(m.config.PollingInterval)
			continue
		}

		if !m.isContentEqual(content, m.lastContent) {
			m.processNewContent(content)
		}

		time.Sleep(m.config.PollingInterval)
	}
}

func (m *Monitor) processNewContent(content *ClipboardContent) {
	m.logger.Info("New clipboard content detected", "type", content.Type)
	
	// Save to local storage
	if err := m.storage.SaveContent(content); err != nil {
		m.logger.Error("Failed to save content to storage", "error", err)
	}

	// Publish to MQTT broker
	if err := m.mqttClient.PublishContent(content); err != nil {
		m.logger.Error("Failed to publish content to broker", "error", err)
	}

	m.lastContent = content
}

func (m *Monitor) isContentEqual(content1, content2 *ClipboardContent) bool {
	if content1 == nil || content2 == nil {
		return content1 == content2
	}
	return content1.Type == content2.Type && content1.Data == content2.Data
}
