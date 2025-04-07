// Package protocol provides protocol-specific implementations for sync functionality
// This file defines the common protocol interfaces and utilities
package protocol

import (
	"fmt"
	"sync"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/sync"
	"github.com/berrythewa/clipman-daemon/internal/types"
)

// MessageType defines common message types for protocol implementations
type MessageType string

// Common message types used across all protocols
const (
	// Content messages
	MessageTypeContent MessageType = "content"  // Regular clipboard content
	MessageTypeFile    MessageType = "file"     // File transfer message
	
	// Control messages
	MessageTypeJoin         MessageType = "join"         // Join group request/notification
	MessageTypeLeave        MessageType = "leave"        // Leave group request/notification
	MessageTypePing         MessageType = "ping"         // Ping request
	MessageTypePong         MessageType = "pong"         // Pong response
	MessageTypeAck          MessageType = "ack"          // Acknowledgment
	MessageTypeHello        MessageType = "hello"        // Introduction when joining a group
	MessageTypeGroupInfo    MessageType = "group_info"   // Group information
	MessageTypeGroupList    MessageType = "group_list"   // List of available groups
	MessageTypeDiscover     MessageType = "discover"     // Peer discovery request
	MessageTypePresence     MessageType = "presence"     // Presence announcement
	
	// Transfer control messages
	MessageTypeTransferRequest  MessageType = "transfer_request"  // Request to initiate a transfer
	MessageTypeTransferAccept   MessageType = "transfer_accept"   // Accept a transfer request
	MessageTypeTransferReject   MessageType = "transfer_reject"   // Reject a transfer request
	MessageTypeTransferProgress MessageType = "transfer_progress" // Transfer progress update
	MessageTypeTransferComplete MessageType = "transfer_complete" // Transfer completion notification
	MessageTypeTransferCancel   MessageType = "transfer_cancel"   // Transfer cancellation
)

// BaseProtocolOptions defines common configuration options for all protocols
type BaseProtocolOptions struct {
	DeviceID         string        // Unique identifier for this device
	DeviceName       string        // Human-readable name for this device
	ConnectTimeout   time.Duration // Timeout for connection attempts
	ReconnectInterval time.Duration // Interval between reconnection attempts
	KeepAliveInterval time.Duration // Interval for keepalive messages
	MaxMessageSize   int64         // Maximum message size in bytes
	DefaultGroup     string        // Default group to join on startup
	EnableCompression bool         // Whether to enable message compression
	EnableEncryption bool          // Whether to enable message encryption
}

// MessageHandler is a function that handles received messages
type MessageHandler func(sync.Message) error

// ContentHandler is a function that handles received clipboard content
type ContentHandler func(*types.ClipboardContent, sync.PeerInfo) error

// GroupHandler is a function that handles group-related events
type GroupHandler func(string, []sync.PeerInfo) error

// PeerHandler is a function that handles peer-related events
type PeerHandler func(sync.PeerInfo, bool) error

// GroupInfo contains information about a group and its members
type GroupInfo struct {
	Name        string         // Group name
	Description string         // Group description
	Members     []sync.PeerInfo // Members of the group
	Created     time.Time      // When the group was created
	IsPrivate   bool           // Whether the group is private
	Password    string         // Optional password for private groups
}

// ProtocolFactory creates protocol implementations based on configuration
type ProtocolFactory interface {
	// Create a protocol implementation with the given configuration
	CreateProtocol(options interface{}) (sync.Protocol, error)
	
	// Check if this factory supports the given configuration
	SupportsConfig(config interface{}) bool
}

// Global registry of protocol factories
var (
	protocolFactories = make(map[string]ProtocolFactory)
	factoryMutex      sync.RWMutex
)

// RegisterProtocolFactory registers a protocol factory
func RegisterProtocolFactory(name string, factory ProtocolFactory) {
	factoryMutex.Lock()
	defer factoryMutex.Unlock()

	protocolFactories[name] = factory
}

// GetProtocolFactory returns a protocol factory by name
func GetProtocolFactory(name string) (ProtocolFactory, error) {
	factoryMutex.RLock()
	defer factoryMutex.RUnlock()

	factory, ok := protocolFactories[name]
	if !ok {
		return nil, fmt.Errorf("protocol factory not found: %s", name)
	}

	return factory, nil
}

// CommonMessageFields contains fields common to all message types
// Protocol-specific implementations can embed this struct
type CommonMessageFields struct {
	Type        string    // Message type
	ID          string    // Unique message ID
	Source      string    // Source device ID
	Destination string    // Destination device ID
	Group       string    // Group ID
	Timestamp   time.Time // Message timestamp
	NeedsAck    bool      // Whether this message requires an acknowledgment
}

// Factory functions to create protocol-specific messages will be added
// by each protocol implementation 

// ProtocolOptions contains common options for all protocols
type ProtocolOptions struct {
	ReconnectDelay    time.Duration // Delay between reconnection attempts
	ReconnectMaxRetry int           // Maximum number of reconnection attempts
}

// Client interface for protocol implementations
type Client interface {
	// Connect connects to the backend
	Connect() error

	// Disconnect disconnects from the backend
	Disconnect() error

	// IsConnected returns whether the client is connected
	IsConnected() bool

	// JoinGroup joins a synchronization group
	JoinGroup(group string) error

	// LeaveGroup leaves a synchronization group
	LeaveGroup(group string) error

	// ListGroups returns the list of joined groups
	ListGroups() []string

	// Send sends a message
	Send(msg sync.Message) error

	// AddHandler adds a message handler
	AddHandler(handler MessageHandler)
}

// CreateContentMessage creates a new content message using the specified factory
func CreateContentMessage(factory ProtocolFactory, content *types.ClipboardContent) (sync.Message, error) {
	// This is a somewhat hacky way to create a content message without knowing the underlying type
	// In a real implementation, we might want to define a more formal interface for message creation
	client, err := factory.NewClient(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create temporary client: %w", err)
	}

	// Try to use reflection to determine the content message type
	// This is a bit of a hack, but it's the best we can do without knowing the concrete type
	// In a real implementation, we might want to define a formal method for creating messages
	var msg sync.Message

	// First, try to call a CreateContentMessage method if it exists
	if creator, ok := factory.(interface {
		CreateContentMessage(*types.ClipboardContent) (sync.Message, error)
	}); ok {
		return creator.CreateContentMessage(content)
	}

	// Fall back to a generic message
	return nil, fmt.Errorf("protocol does not support content message creation")
} 