// Package sync provides clipboard synchronization capabilities
package types

import (
    "time"

    "github.com/berrythewa/clipman-daemon/internal/types"
    "github.com/libp2p/go-libp2p/core/peer"
)

// PeerInfo contains information about a discovered peer
type PeerInfo struct {
    ID         peer.ID    // libp2p peer ID
    Name       string     // Human-readable name
    Addrs      []string   // MultiAddrs for connecting to this peer
    Groups     []string   // Groups this peer belongs to
    LastSeen   time.Time  // When this peer was last seen
    DeviceType string     // Type of device (desktop, mobile, etc.)
}

// SyncStatus provides information about the current sync status
type SyncStatus struct {
    Connected    bool      // Whether connected to any peers
    PeerCount    int       // Number of connected peers
    ActiveGroups []string  // Currently joined groups
}

// ContentCallback defines the function signature for content handlers
type ContentCallback func(*types.ClipboardContent, PeerInfo)

// SyncManager defines the interface for the sync functionality
type SyncManager interface {
    // Core lifecycle methods
    Start() error
    Stop() error
    
    // Content operations
    SendContent(content *types.ClipboardContent, group string) error
    SetContentHandler(handler ContentCallback)
    
    // Group management
    JoinGroup(group string) error
    LeaveGroup(group string) error
    ListGroups() ([]string, error)
    
    // Discovery management
    GetDiscoveredPeers() []PeerInfo
    
    // Connection status
    IsConnected() bool
}