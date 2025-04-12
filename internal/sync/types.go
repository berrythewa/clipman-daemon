// Package sync provides clipboard synchronization using libp2p
package sync

import (
	"time"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/libp2p/go-libp2p/core/peer"
)

// InternalPeerInfo contains detailed information about a discovered peer
// Used internally by the sync package
type InternalPeerInfo struct {
	ID           string            // Peer ID (string representation of peer.ID)
	Name         string            // Human-readable name
	Addrs        []string          // Multiaddrs for connecting to the peer
	Groups       []string          // Groups the peer is a member of
	LastSeen     time.Time         // When the peer was last seen
	Capabilities map[string]string // Peer capabilities
	Version      string            // Software version
	DeviceType   string            // Type of device (desktop, mobile, etc.)
}

// ToExternalPeerInfo converts an internal peer info to the external type
func (p InternalPeerInfo) ToExternalPeerInfo() types.PeerInfo {
	return types.PeerInfo{
		ID:         p.ID,
		Name:       p.Name,
		DeviceType: p.DeviceType,
		LastSeen:   p.LastSeen,
	}
}

// FromExternalPeerInfo creates an internal peer info from the external type
func FromExternalPeerInfo(p types.PeerInfo) InternalPeerInfo {
	return InternalPeerInfo{
		ID:         p.ID,
		Name:       p.Name,
		DeviceType: p.DeviceType,
		LastSeen:   p.LastSeen,
		// Default values for the other fields
		Capabilities: make(map[string]string),
	}
}

// SyncMessage represents a message exchanged between peers
type SyncMessage struct {
	Type        string            // Message type (content, control, etc.)
	Source      peer.ID           // Source peer ID
	Destination peer.ID           // Destination peer ID (empty for broadcast)
	Group       string            // Target group (if applicable)
	Timestamp   time.Time         // Message timestamp
	ID          string            // Unique message ID
	Payload     []byte            // Message payload
	Headers     map[string]string // Message headers/metadata
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

// SyncError wraps errors from the sync package
type SyncError struct {
	Code    int    // Error code
	Message string // Error message
	Err     error  // Original error
}

// Error implements the error interface
func (e *SyncError) Error() string {
	if e.Err != nil {
		return e.Message + ": " + e.Err.Error()
	}
	return e.Message
}

// Unwrap returns the wrapped error
func (e *SyncError) Unwrap() error {
	return e.Err
}

// NewSyncError creates a new SyncError
func NewSyncError(code int, message string, err error) *SyncError {
	return &SyncError{
		Code:    code,
		Message: message,
		Err:     err,
	}
} 