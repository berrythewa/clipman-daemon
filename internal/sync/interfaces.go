// Package sync provides synchronization capabilities for clipman content
// This file defines the core interfaces for the sync package
package sync

import (
	"io"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/types"
)

// SyncManager is the main entry point for the sync package
// It provides high-level operations for applications to use
type SyncManager interface {
	// Core lifecycle methods
	Start() error
	Stop() error
	Status() SyncStatus

	// Content operations
	SendContent(content *types.ClipboardContent, group string) error
	SetContentHandler(handler func(*types.ClipboardContent, PeerInfo))

	// Group management
	JoinGroup(group string) error
	LeaveGroup(group string) error
	ListGroups() ([]string, error)

	// Discovery management
	EnableDiscovery(enabled bool) error
	GetDiscoveredPeers() []PeerInfo
	
	// Connection status
	IsConnected() bool
}

// Protocol defines the interface for sync protocol implementations
// It abstracts the details of specific protocols (MQTT, P2P)
type Protocol interface {
	// Lifecycle management
	Initialize() error
	Start() error
	Stop() error

	// Connection management
	IsConnected() bool
	Connect() error
	Disconnect() error

	// Messaging
	SendMessage(msg Message) error
	SetMessageHandler(handler func(Message))

	// Group management
	JoinGroup(group string) error
	LeaveGroup(group string) error
	ListGroups() ([]string, error)
}

// Discovery defines the interface for peer discovery mechanisms
// It handles finding and announcing peers on the network
type Discovery interface {
	// Lifecycle management
	Start() error
	Stop() error

	// Discovery operations
	Announce(info PeerInfo) error
	SetPeerDiscoveryHandler(handler func(PeerInfo))
	GetDiscoveredPeers() []PeerInfo
	
	// Advanced discovery options
	EnableMDNS(enabled bool) error
	EnableMQTTDiscovery(enabled bool) error
}

// Message is the interface for all sync messages
// It defines the structure of messages exchanged between peers
type Message interface {
	// Message metadata
	Type() string
	Payload() []byte
	Source() string
	Destination() string
	Group() string
	Timestamp() time.Time
	ID() string
	
	// Message control
	SetDestination(dest string)
	SetGroup(group string)
	RequestAck() bool
}

// Transport defines the interface for network transport implementations
// It handles the actual data transmission
type Transport interface {
	// Connection management
	Connect(address string) error
	Disconnect() error
	IsConnected() bool
	
	// Data transmission
	SendData(data []byte, dest string) error
	SetDataHandler(handler func([]byte, string))
	
	// Resource management
	Close() error
}

// Security defines the interface for security operations
// It handles encryption, authentication, and integrity
type Security interface {
	// Encryption
	Encrypt(data []byte, recipient string) ([]byte, error)
	Decrypt(data []byte, sender string) ([]byte, error)
	
	// Authentication
	Authenticate(peerID string, credentials []byte) (bool, error)
	GenerateCredentials() ([]byte, error)
	
	// Key management
	GenerateKeyPair() error
	ImportPublicKey(peerID string, key []byte) error
	ExportPublicKey() ([]byte, error)
}

// File transfer interfaces
type FileTransfer interface {
	// File operations
	SendFile(filePath string, recipient string, group string) error
	ReceiveFile(handler func(io.Reader, FileInfo) error)
	CancelTransfer(transferID string) error
	
	// Transfer status
	ListTransfers() []TransferInfo
	GetTransferProgress(transferID string) (TransferProgress, error)
}

// PeerInfo contains information about a discovered peer
type PeerInfo struct {
	ID           string            // Unique identifier for the peer
	Name         string            // Human-readable name
	Address      string            // Network address
	Groups       []string          // Sync groups the peer belongs to
	LastSeen     time.Time         // Last time the peer was seen
	Capabilities map[string]string // Capabilities of the peer
	Version      string            // Software version
	DeviceType   string            // Type of device (desktop, mobile, server)
}

// SyncStatus provides information about the current sync status
type SyncStatus struct {
	Connected    bool      // Whether connected to any peers or brokers
	Mode         string    // "p2p", "mqtt", or "hybrid"
	ActiveGroups []string  // Currently joined groups
	PeerCount    int       // Number of connected peers
	Stats        SyncStats // Statistical information
}

// SyncStats provides statistical information about sync operations
type SyncStats struct {
	BytesSent        int64     // Total bytes sent
	BytesReceived    int64     // Total bytes received
	MessagesSent     int       // Total messages sent
	MessagesReceived int       // Total messages received
	LastSyncTime     time.Time // Last successful sync operation time
	Errors           int       // Total error count
	SuccessRate      float64   // Percentage of successful operations
}

// GroupInfo provides information about a sync group
type GroupInfo struct {
	Name        string    // Group name
	Description string    // Group description
	Members     []string  // Member peer IDs
	Created     time.Time // When the group was created
	IsPrivate   bool      // Whether the group is private
	Password    string    // Optional password for private groups
}

// FileInfo provides information about a file being transferred
type FileInfo struct {
	Name        string    // Filename
	Size        int64     // File size in bytes
	ModTime     time.Time // Modification time
	ContentType string    // MIME type
	Hash        string    // File hash for verification
}

// TransferInfo provides information about a file transfer
type TransferInfo struct {
	ID          string    // Unique transfer ID
	FileName    string    // Name of the file
	FileSize    int64     // Size of the file in bytes
	Sender      string    // Sender peer ID
	Recipient   string    // Recipient peer ID
	Group       string    // Group context (if applicable)
	StartTime   time.Time // When the transfer started
	IsIncoming  bool      // Whether this is an incoming or outgoing transfer
	Status      string    // "pending", "in_progress", "completed", "failed", "cancelled"
}

// TransferProgress provides progress information for a file transfer
type TransferProgress struct {
	ID               string  // Transfer ID
	BytesTransferred int64   // Number of bytes transferred
	TotalBytes       int64   // Total bytes to transfer
	PercentComplete  float64 // Percentage complete (0-100)
	CurrentSpeed     int64   // Current transfer speed in bytes/sec
	TimeRemaining    int64   // Estimated time remaining in seconds
	Status           string  // Current status
} 