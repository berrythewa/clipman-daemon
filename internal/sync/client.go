package sync

import (
	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/types"
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

// CreateClient creates a new SyncClient with the configured mode
func CreateClient(cfg *config.Config, logger *zap.Logger) (SyncClient, error) {
	// Ensure config compatibility
	if cfg.Sync.URL == "" && cfg.Broker.URL != "" {
		// Use legacy broker settings if sync URL is not set
		cfg.Sync.URL = cfg.Broker.URL
		cfg.Sync.Username = cfg.Broker.Username
		cfg.Sync.Password = cfg.Broker.Password
	}
	
	// Create the appropriate client based on the sync mode
	if cfg.Sync.IsModeCentralized() {
		// Centralized mode uses MQTT client
		return NewMQTTClient(cfg, logger)
	} else {
		// P2P mode also uses MQTT for now, but with local discovery
		return NewMQTTClient(cfg, logger)
	}
} 