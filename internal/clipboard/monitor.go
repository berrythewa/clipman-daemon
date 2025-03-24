package clipboard

import (
	"bytes"
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/broker"
	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/storage"
	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/berrythewa/clipman-daemon/pkg/utils"
)

type Monitor struct {
	config           *config.Config
	mqttClient       broker.MQTTClientInterface
	logger           *utils.Logger
	clipboard        Clipboard
	storage          *storage.BoltStorage
	lastContent      *types.ClipboardContent
	history          *ClipboardHistory
	mu               sync.Mutex
	ctx              context.Context
	cancel           context.CancelFunc
	contentProcessor *ContentProcessor
}

func NewMonitor(cfg *config.Config, mqttClient broker.MQTTClientInterface, logger *utils.Logger, storage *storage.BoltStorage) *Monitor {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Monitor{
		config:           cfg,
		mqttClient:       mqttClient,
		logger:           logger,
		clipboard:        NewClipboard(),
		storage:          storage,
		history:          NewClipboardHistory(100), // Keep last 100 items
		ctx:              ctx,
		cancel:           cancel,
		contentProcessor: NewContentProcessor(),
	}

	// Add some default filters and transformers
	m.contentProcessor.AddFilter(LengthFilter(1000))
	m.contentProcessor.AddTransformer(TrimTransformer())
	return m
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

func (m *Monitor) Stop() error {
	m.logger.Info("Stopping clipboard monitor")
	m.cancel()
	return nil
}

func (m *Monitor) monitorClipboard() {
	m.logger.Info("Starting clipboard monitor using platform-specific implementation")
	
	// Create channels for communication
	contentCh := make(chan *types.ClipboardContent, 10)
	stopCh := make(chan struct{})
	
	// Start monitoring using the platform-specific implementation
	m.clipboard.MonitorChanges(contentCh, stopCh)
	
	// Process incoming clipboard content
	for {
		select {
		case <-m.ctx.Done():
			close(stopCh)
			return
		case content := <-contentCh:
			if content == nil {
				m.logger.Debug("Received nil content, skipping")
				continue
			}
			
			m.logger.Debug("Received clipboard content from platform monitor",
				"type", content.Type,
				"size", len(content.Data),
				"data_preview", string(content.Data[:min(len(content.Data), 50)]))
			
			m.mu.Lock()
			if !m.isContentEqual(content, m.lastContent) {
				m.logger.Debug("Processing new content")
				m.processNewContent(content)
			} else {
				m.logger.Debug("Content is equal to previous, skipping")
			}
			m.mu.Unlock()
		}
	}
}

// checksumData creates a simple checksum of data
func checksumData(data []byte) []byte {
	if len(data) == 0 {
		return nil
	}
	
	// Simple XOR-based checksum of the data
	var checksum byte
	for _, b := range data {
		checksum ^= b
	}
	
	return []byte{checksum}
}

func (m *Monitor) processNewContent(content *types.ClipboardContent) {
	m.logger.Info("New clipboard content detected", "type", content.Type)
	m.logger.Debug("Raw content received",
		"type", content.Type,
		"data_length", len(content.Data),
		"raw_data", string(content.Data[:min(len(content.Data), 50)]))

	content = m.contentProcessor.Process(content)
	if content == nil {
		m.logger.Info("Content filtered out")
		return
	}

	content = m.prepareContent(content)

	if err := m.saveContent(content); err != nil {
		m.logger.Error("Failed to save content", "error", err)
	}

	if err := m.publishContent(content); err != nil {
		m.logger.Error("Failed to publish content", "error", err)
	}

	m.history.Add(content)
	m.lastContent = content

	m.logger.Info("--- Current Clipboard History ---")
	history := m.history.GetLast(5)
	for i, item := range history {
		// Format content based on type
		var displayContent string
		switch item.Content.Type {
		case types.TypeImage:
			displayContent = fmt.Sprintf("[Image Data: %d bytes]", len(item.Content.Data))
		case types.TypeFile:
			displayContent = fmt.Sprintf("[File: %s]", string(item.Content.Data))
		case types.TypeURL:
			displayContent = fmt.Sprintf("[URL: %s]", string(item.Content.Data))
		default:
			// For text, show preview
			if len(item.Content.Data) > 50 {
				displayContent = fmt.Sprintf("%s...", string(item.Content.Data[:50]))
			} else {
				displayContent = string(item.Content.Data)
			}
		}

		m.logger.Info(fmt.Sprintf("History item %d:", i+1),
			"type", item.Content.Type,
			"time", item.Time.Format("15:04:05"),
			"content", displayContent)
	}
	m.logger.Info("----------------------------")
}

func (m *Monitor) prepareContent(content *types.ClipboardContent) *types.ClipboardContent {
	content.Type = detectContentType(content.Data)
	// compressedContent, err := compression.CompressContent(content)
	// if err != nil {
	// 	m.logger.Error("Failed to compress content", "error", err)
	// 	return content
	// }
	// return compressedContent
	return content
}

func (m *Monitor) saveContent(content *types.ClipboardContent) error {
	return m.storage.SaveContent(content)
}

func (m *Monitor) publishContent(content *types.ClipboardContent) error {
	return m.mqttClient.PublishContent(content)
}

func (m *Monitor) isContentEqual(content1, content2 *types.ClipboardContent) bool {
	if content1 == nil || content2 == nil {
		return content1 == content2
	}

	// Basic content equality - compare type and data
	isEqual := content1.Type == content2.Type && bytes.Equal(content1.Data, content2.Data)

	if !isEqual {
		// Only log at debug level to avoid spamming
		if len(content2.Data) > 0 && len(content1.Data) > 0 {
			m.logger.Debug("Content changed",
				"old_type", content2.Type,
				"new_type", content1.Type,
				"old_length", len(content2.Data),
				"new_length", len(content1.Data))
		}
		return false
	}

	// Even if content is the same, check if it's a new copy action
	// But with a higher threshold to avoid false positives
	if content1.Created.Sub(content2.Created) > 5*time.Second {
		m.logger.Debug("Same content copied again after timeout",
			"time_diff", content1.Created.Sub(content2.Created))
		return false
	}

	return true
}

// New method to get clipboard history
func (m *Monitor) GetHistory(n int) []*HistoryItem {
	return m.history.GetLast(n)
}
