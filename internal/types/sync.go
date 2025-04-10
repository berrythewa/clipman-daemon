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

// Node manages the libp2p host and networking
type Node struct {
    host   host.Host
    ctx    context.Context
    cancel context.CancelFunc
    logger *zap.Logger
}

// SyncConfig holds configuration for the sync functionality
type SyncConfig struct {
    // Core Sync Settings
    Enabled           bool     `json:"enable_sync"`
    SyncOverInternet  bool     `json:"sync_over_internet"`
    UseRelayNodes     bool     `json:"use_relay_nodes"`
    ListenPort        int      `json:"listen_port"`
    PeerIdentity      string   `json:"peer_identity"`
    DiscoveryMethod   string   `json:"discovery_method"` // "mdns", "dht", or "manual"
    
    // Clipboard Sync Options
    ClipboardTypes    []string `json:"clipboard_types"`  // "text", "image", "files"
    AutoCopyFromPeers bool     `json:"auto_copy_from_peers"`
    MaxClipboardSizeKB int     `json:"max_clipboard_size_kb"`
    ClipboardHistorySize int   `json:"clipboard_history_size"`
    ClipboardBlacklistApps []string `json:"clipboard_blacklist_apps"`
    
    // File Transfer Options
    EnableFileSharing bool     `json:"enable_file_sharing"`
    RequireFileConfirmation bool `json:"require_file_confirmation"`
    DefaultDownloadFolder string `json:"default_download_folder"`
    AutoAcceptFromPeers []string `json:"auto_accept_from_peers"`
    MaxFileSizeMB      int     `json:"max_file_size_mb"`
    
    // Privacy & Security
    AllowOnlyKnownPeers bool   `json:"allow_only_known_peers"`
    TrustedPeers       []string `json:"trusted_peers"`
    RequireApprovalPin bool    `json:"require_approval_pin"`
    LogPeerActivity    bool    `json:"log_peer_activity"`
    
    // Developer & Debug Options
    DebugLogging       bool    `json:"debug_logging"`
    ShowPeerDebugInfo  bool    `json:"show_peer_debug_info"`
    DisableMultiplexing bool   `json:"disable_multiplexing"`
    ForceDirectConnectionOnly bool `json:"force_direct_connection_only"`
}