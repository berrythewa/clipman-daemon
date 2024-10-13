package clipboard

import (
    "context"
    "sync"
    "time"
    "bytes"

    "github.com/berrythewa/clipman-daemon/internal/config"
    "github.com/berrythewa/clipman-daemon/internal/broker"
    "github.com/berrythewa/clipman-daemon/internal/storage"
    "github.com/berrythewa/clipman-daemon/internal/types"
    "github.com/berrythewa/clipman-daemon/pkg/utils"
    "github.com/berrythewa/clipman-daemon/pkg/compression"

)

type Monitor struct {
	config      *config.Config
	mqttClient  *broker.MQTTClient
	logger      *utils.Logger
	clipboard   Clipboard
	storage     *storage.BoltStorage
    lastContent *types.ClipboardContent
	history     *ClipboardHistory
	mu          sync.Mutex
	ctx         context.Context
	cancel      context.CancelFunc
	contentProcessor *ContentProcessor

}

func NewMonitor(cfg *config.Config, mqttClient *broker.MQTTClient, logger *utils.Logger, storage *storage.BoltStorage) *Monitor {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Monitor{
		config:     cfg,
		mqttClient: mqttClient,
		logger:     logger,
		clipboard:  NewClipboard(),
		storage:    storage,
		history:    NewClipboardHistory(100), // Keep last 100 items
		ctx:        ctx,
		cancel:     cancel,
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
	ticker := time.NewTicker(m.config.PollingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			content, err := m.clipboard.Read()
			if err != nil {
				m.logger.Error("Error reading clipboard", "error", err)
				continue
			}

			m.mu.Lock()
			if !m.isContentEqual(content, m.lastContent) {
				m.processNewContent(content)
			}
			m.mu.Unlock()
		}
	}
}

func (m *Monitor) processNewContent(content *types.ClipboardContent) {
	m.logger.Info("New clipboard content detected", "type", content.Type)
	
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
}

func (m *Monitor) prepareContent(content *types.ClipboardContent) *types.ClipboardContent {
    content.Type = detectContentType(content.Data)
    compressedContent, err := compression.compressContent(content)
    if err != nil {
        m.logger.Error("Failed to compress content", "error", err)
        return content
    }
    return compressedContent
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
	return content1.Type == content2.Type && bytes.Equal(content1.Data, content2.Data)
}


// New method to get clipboard history
func (m *Monitor) GetHistory(n int) []*HistoryItem {
	return m.history.GetLast(n)
}