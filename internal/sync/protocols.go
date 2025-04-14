// Package sync provides clipboard synchronization using libp2p
package sync

import (
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"go.uber.org/zap"
)

// Protocol paths
const (
	// Protocol versions
	ProtocolVersion = "1.0.0"

	// Base protocol path
	BaseProtocolPath = "/clipman/"

	// Full protocol paths
	PairingProtocolPath   = BaseProtocolPath + ProtocolVersion + "/pairing"
	ClipboardProtocolPath = BaseProtocolPath + ProtocolVersion + "/clipboard"
	FileProtocolPath      = BaseProtocolPath + ProtocolVersion + "/file"
)

// ProtocolHandler defines the interface for protocol-specific handlers
type ProtocolHandler interface {
	// ID returns the protocol ID
	ID() protocol.ID

	// Handle processes an incoming stream
	Handle(stream network.Stream)

	// Start initializes and starts the protocol handler
	Start() error

	// Stop stops the protocol handler
	Stop() error
}

// ProtocolManager manages the various protocols used for peer communication
type ProtocolManager struct {
	// Host and context
	host   host.Host
	ctx    context.Context
	cancel context.CancelFunc
	logger *zap.Logger
	config *SyncConfig

	// State
	started  bool
	handlers map[protocol.ID]ProtocolHandler
}

// NewProtocolManager creates a new protocol manager
func NewProtocolManager(ctx context.Context, host host.Host, config *SyncConfig, logger *zap.Logger) *ProtocolManager {
	pmCtx, cancel := context.WithCancel(ctx)

	pm := &ProtocolManager{
		host:     host,
		ctx:      pmCtx,
		cancel:   cancel,
		logger:   logger.With(zap.String("component", "protocol-manager")),
		config:   config,
		started:  false,
		handlers: make(map[protocol.ID]ProtocolHandler),
	}

	return pm
}

// AddHandler adds a protocol handler
func (pm *ProtocolManager) AddHandler(handler ProtocolHandler) {
	protocolID := handler.ID()
	pm.handlers[protocolID] = handler
}

// Start starts all protocol handlers
func (pm *ProtocolManager) Start() error {
	if pm.started {
		return nil
	}

	for id, handler := range pm.handlers {
		pm.host.SetStreamHandler(id, handler.Handle)
		
		if err := handler.Start(); err != nil {
			pm.logger.Error("Failed to start protocol handler",
				zap.String("protocol", string(id)),
				zap.Error(err))
			continue
		}
		
		pm.logger.Info("Started protocol handler", zap.String("protocol", string(id)))
	}

	pm.started = true
	return nil
}

// Stop stops all protocol handlers
func (pm *ProtocolManager) Stop() error {
	if !pm.started {
		return nil
	}

	for id, handler := range pm.handlers {
		pm.host.RemoveStreamHandler(id)
		
		if err := handler.Stop(); err != nil {
			pm.logger.Error("Failed to stop protocol handler",
				zap.String("protocol", string(id)),
				zap.Error(err))
		}
	}

	pm.cancel()
	pm.started = false
	return nil
}

// GetHandler returns the handler for a specific protocol
func (pm *ProtocolManager) GetHandler(id protocol.ID) (ProtocolHandler, error) {
	handler, exists := pm.handlers[id]
	if !exists {
		return nil, fmt.Errorf("no handler registered for protocol %s", id)
	}
	return handler, nil
}

// OpenStream opens a stream to a peer using a specific protocol
func (pm *ProtocolManager) OpenStream(peerID peer.ID, protocolID protocol.ID) (network.Stream, error) {
	// Get the handler to make sure it's registered
	_, err := pm.GetHandler(protocolID)
	if err != nil {
		return nil, err
	}

	// Open the stream
	stream, err := pm.host.NewStream(pm.ctx, peerID, protocolID)
	if err != nil {
		return nil, fmt.Errorf("failed to open stream to peer %s: %w", peerID.String(), err)
	}

	return stream, nil
}

// Common IO helpers for protocols

// writeBytes writes bytes to a stream
func writeBytes(stream network.Stream, data []byte) error {
	if stream == nil {
		return errors.New("nil stream")
	}
	
	n, err := stream.Write(data)
	if err != nil {
		return err
	}
	
	if n != len(data) {
		return fmt.Errorf("failed to write all bytes: wrote %d out of %d", n, len(data))
	}
	
	return nil
}

// readBytes reads exactly len(data) bytes from a stream
func readBytes(stream network.Stream, data []byte) error {
	if stream == nil {
		return errors.New("nil stream")
	}
	
	bytesRead := 0
	for bytesRead < len(data) {
		n, err := stream.Read(data[bytesRead:])
		if err != nil {
			if err == io.EOF && bytesRead+n == len(data) {
				bytesRead += n
				break
			}
			return err
		}
		bytesRead += n
	}
	
	return nil
} 