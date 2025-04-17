package clipboard

import (
	"github.com/berrythewa/clipman-daemon/internal/types"
	"go.uber.org/zap"
)

// NoOpPublisher implements ContentPublisher interface but does nothing
// It's used as a placeholder when sync functionality is disabled
type NoOpPublisher struct {
	logger *zap.Logger
}

// NewNoOpPublisher creates a new NoOpPublisher instance
func NewNoOpPublisher(logger *zap.Logger) *NoOpPublisher {
	return &NoOpPublisher{
		logger: logger,
	}
}

// PublishContent implements ContentPublisher.PublishContent
// It logs the event but does not actually publish the content
func (p *NoOpPublisher) PublishContent(content *types.ClipboardContent) error {
	if p.logger != nil {
		p.logger.Debug("NoOpPublisher: content publishing skipped (no-op)",
			zap.String("content_type", string(content.Type)),
			zap.Int("content_size", len(content.Data)))
	}
	return nil
}

// IsConnected implements ContentPublisher.IsConnected
// Always returns true to avoid sync-related errors
func (p *NoOpPublisher) IsConnected() bool {
	return true
} 