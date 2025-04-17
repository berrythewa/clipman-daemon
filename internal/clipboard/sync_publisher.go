package clipboard

import (
	"github.com/berrythewa/clipman-daemon/internal/sync"
	"github.com/berrythewa/clipman-daemon/internal/types"
	"go.uber.org/zap"
)

// SyncPublisher implements ContentPublisher interface using the sync package
type SyncPublisher struct {
	syncManager *sync.Manager
	groupName   string
	logger      *zap.Logger
}

// NewSyncPublisher creates a new SyncPublisher that uses the sync manager
func NewSyncPublisher(syncManager *sync.Manager, groupName string, logger *zap.Logger) *SyncPublisher {
	return &SyncPublisher{
		syncManager: syncManager,
		groupName:   groupName,
		logger:      logger,
	}
}

// PublishContent implements ContentPublisher.PublishContent
// It publishes the content to other devices using the sync manager
func (p *SyncPublisher) PublishContent(content *types.ClipboardContent) error {
	if p.syncManager == nil {
		if p.logger != nil {
			p.logger.Debug("SyncPublisher: sync manager is nil, skipping content publishing")
		}
		return nil
	}

	if !p.IsConnected() {
		if p.logger != nil {
			p.logger.Debug("SyncPublisher: not connected to any peers, skipping content publishing")
		}
		return nil
	}

	if p.logger != nil {
		p.logger.Debug("SyncPublisher: publishing content",
			zap.String("content_type", string(content.Type)),
			zap.Int("content_size", len(content.Data)),
			zap.String("group", p.groupName))
	}

	return p.syncManager.SendContent(content, p.groupName)
}

// IsConnected implements ContentPublisher.IsConnected
// Checks if the sync manager is connected to any peers
func (p *SyncPublisher) IsConnected() bool {
	if p.syncManager == nil {
		return false
	}
	return p.syncManager.IsConnected()
} 