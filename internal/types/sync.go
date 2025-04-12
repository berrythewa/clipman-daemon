// Package types defines common types used throughout the application
package types

import (
	"time"
)

// PeerInfo contains basic information about a peer for display or reference
type PeerInfo struct {
	ID         string    // Peer ID 
	Name       string    // Human-readable name
	DeviceType string    // Type of device (desktop, mobile, etc.)
	LastSeen   time.Time // When this peer was last seen
}

// SyncStatus provides information about the current sync status
type SyncStatus struct {
	Connected    bool      // Whether connected to any peers
	PeerCount    int       // Number of connected peers
	ActiveGroups []string  // Currently joined groups
}

// ContentCallback defines the function signature for content handlers
type ContentCallback func(*ClipboardContent, PeerInfo)

// SyncManager defines the interface for the sync functionality
type SyncManager interface {
	// Core lifecycle methods
	Start() error
	Stop() error
	
	// Content operations
	SendContent(content *ClipboardContent, group string) error
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