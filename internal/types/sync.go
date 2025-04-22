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

// PairingRequest contains information about a device requesting pairing
type PairingRequest struct {
	DeviceName string // Human-readable name of the requesting device
	DeviceType string // Type of device (desktop, mobile, etc.)
	PeerID     string // Peer ID of the requester
}

// PairingResponse is the response to a pairing request
type PairingResponse struct {
	Accepted     bool   // Whether the pairing was accepted
	ErrorMessage string // Error message if rejected
	PairingCode  string // Verification code to display to the user
	DeviceName   string // Name of the responding device
	DeviceType   string // Type of the responding device
	PeerID       string // Peer ID of the responder
}

// PairedDevice represents a device that has been paired
type PairedDevice struct {
	PeerID     string    // Peer ID of the paired device
	DeviceName string    // Human-readable name
	DeviceType string    // Type of device (desktop, mobile, etc.)
	LastSeen   time.Time // When this device was last seen
	PairedAt   time.Time // When the pairing was established
}

// PairingRequestCallback is called when a pairing request is received
type PairingRequestCallback func(request PairingRequest, remotePeerId string) (bool, error)

// SyncManager defines the interface for the sync functionality
type SyncManager interface {
	// Core lifecycle methods
	Start() error
	Stop() error
	GetConfig() *SyncConfig
	
	// Content operations
	SendContent(content *ClipboardContent, group string) error
	SetContentHandler(handler ContentCallback)
	
	// Group management
	JoinGroup(group string) error
	LeaveGroup(group string) error
	ListGroups() ([]string, error)
	
	// Discovery management
	GetDiscoveredPeers() []PeerInfo
	
	// Pairing operations
	EnablePairing(handler PairingRequestCallback) (string, error)
	DisablePairing()
	RequestPairing(address string) (*PairingResponse, error)
	IsPairingEnabled() bool
	IsPaired(peerID string) bool
	GetPairedDevices() []PairedDevice
	RemovePairedDevice(peerID string) error
	
	// Connection status
	IsConnected() bool
}

// SyncConfig holds a subset of sync configuration for application use
type SyncConfig struct {
	// Core Settings
	DeviceName		  string
	Enabled           bool     
	SyncOverInternet  bool     
	UseRelayNodes     bool     
	ListenPort        int      
	DiscoveryMethod   string
	PairingEnabled	  bool
	
	// Clipboard Options
	ClipboardTypes         []string 
	AutoCopyFromPeers      bool     
	MaxClipboardSizeKB     int      
	ClipboardHistorySize   int      
	ClipboardBlacklistApps []string 
	
	// File Transfer Options
	EnableFileSharing       bool   
	RequireFileConfirmation bool   
	DefaultDownloadFolder   string 
	MaxFileSizeMB           int   
	
	// Privacy & Security
	AllowOnlyKnownPeers bool    
	TrustedPeers        []string
	RequireApprovalPin  bool    
	LogPeerActivity     bool    
	
	// Developer & Debug Options
	DebugLogging              bool 
	ShowPeerDebugInfo         bool 
	DisableMultiplexing       bool 
	ForceDirectConnectionOnly bool 
}