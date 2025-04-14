# Clipman Sync Package

This package provides clipboard synchronization capabilities for Clipman, allowing content to be synced across multiple devices using libp2p for peer-to-peer communication.

## Architecture

The sync package follows a node-centric architecture leveraging libp2p for all networking capabilities:

```
                           ┌───────────────────┐
                           │    Application    │
                           └─────────┬─────────┘
                                     │
                                     ▼
                           ┌───────────────────┐
                           │      sync.go      │
                           └─────────┬─────────┘
                                     │
                                     ▼
                           ┌───────────────────┐
                           │      node.go      │
                           └─────────┬─────────┘
                                     │
                    ┌────────────────┼────────────────┐
                    │                │                │
          ┌─────────▼─────────┐ ┌────▼──────────┐ ┌───▼────────────┐
          │   protocols.go    │ │ discovery.go  │ │  clipboard.go  │
          └───────────────────┘ └───────────────┘ └────────────────┘
                   │                                      │
                   │                                ┌─────▼─────┐
                   │                                │  file.go  │
                   ▼                                └───────────┘
          ┌────────────────┐
          │   pairing.go   │
          └────────────────┘
```

### Key Components

1. **sync.go**: The primary entry point and orchestrator for synchronization. Provides high-level API for applications.
2. **node.go**: Core libp2p node setup, managing the host, identity, and connections.
3. **protocols.go**: Defines and manages custom protocols for clipboard and file synchronization.
4. **discovery.go**: Handles peer discovery using multiple methods (mDNS, DHT, and manual).
5. **pairing.go**: Implements secure device pairing protocol and management.
6. **clipboard.go**: Implements clipboard content synchronization logic.
7. **file.go**: Implements file transfer functionality.
8. **types.go**: Common types and interfaces used throughout the package.

## libp2p Integration

The sync package leverages libp2p for all networking capabilities:

1. **Direct P2P Communication**: Content is transferred directly between peers without central servers
2. **Multiple Transport Support**: Uses libp2p's transport layer (TCP, QUIC, WebRTC)
3. **NAT Traversal**: Automatic NAT hole punching and relay support for connectivity
4. **Secure Communication**: End-to-end encryption with libp2p's secure transports
5. **PubSub**: Group messaging using libp2p's GossipSub
6. **DHT**: Distributed peer discovery for wide-area networking
7. **mDNS**: Local network discovery
8. **Custom Protocols**: Application-specific protocols for pairing and sync

## Directory Structure

```
internal/sync/
├── sync.go             # High-level sync manager API (entrypoint)
├── node.go             # Core libp2p node setup (host, discovery, routing)
├── protocols.go        # Define custom protocols (e.g., clipboard, file)
├── discovery.go        # mDNS / DHT / Manual discovery logic
├── pairing.go          # Secure device pairing implementation
├── clipboard.go        # Clipboard sync handler
├── file.go             # File transfer logic
├── config.go           # Configuration handling
└── types.go            # Common structs/enums
```

## Usage

### Basic Usage

```go
import (
    "github.com/berrythewa/clipman-daemon/internal/config"
    "github.com/berrythewa/clipman-daemon/internal/sync"
    "github.com/berrythewa/clipman-daemon/internal/types"
    "go.uber.org/zap"
)

func main() {
    // Get configuration and logger
    cfg := config.Load("/path/to/config.json")
    logger := zap.NewExample()

    // Create sync manager
    syncManager, err := sync.New(context.Background(), cfg, logger)
    if err != nil {
        logger.Fatal("Failed to create sync manager", zap.Error(err))
    }

    // Start sync manager
    if err := syncManager.Start(); err != nil {
        logger.Fatal("Failed to start sync manager", zap.Error(err))
    }
    defer syncManager.Stop()

    // Join a sync group
    if err := syncManager.JoinGroup("my-group"); err != nil {
        logger.Error("Failed to join group", zap.Error(err))
    }

    // Handle incoming content
    syncManager.SetContentHandler(func(content *types.ClipboardContent, peer types.PeerInfo) {
        logger.Info("Received content",
            zap.String("content_type", string(content.Type)),
            zap.Int("content_size", len(content.Data)),
            zap.String("from_peer", peer.ID))
        
        // Process the content...
    })

    // Send content
    content := &types.ClipboardContent{
        Type: types.TypeText,
        Data: []byte("Hello, world!"),
    }
    
    if err := syncManager.SendContent(content, "my-group"); err != nil {
        logger.Error("Failed to send content", zap.Error(err))
    }
    
    // Manually add a peer
    peerAddr := "/ip4/192.168.1.100/tcp/45678/p2p/QmHashOfThePeer"
    if err := syncManager.AddPeer(peerAddr); err != nil {
        logger.Error("Failed to add peer", zap.Error(err))
    }
}
```

### Secure Device Pairing

```go
// Enter pairing mode on Device A
address, err := syncManager.EnablePairing(func(request types.PairingRequest, remotePeerId string) (bool, error) {
    // This callback will be called when a pairing request is received
    // Here you would typically ask the user if they want to accept the pairing
    // For CLI usage, you could print the request info and read user input
    
    fmt.Printf("Pairing request from %s (%s)\n", request.DeviceName, remotePeerId)
    fmt.Printf("Accept? [y/n]: ")
    
    var response string
    fmt.Scanln(&response)
    
    return strings.ToLower(response) == "y", nil
})

if err != nil {
    log.Fatal("Failed to enable pairing mode:", err)
}

fmt.Println("Share this address with the device you want to pair with:")
fmt.Println(address)

// On Device B, request pairing with the address from Device A
response, err := syncManager.RequestPairing(address)
if err != nil {
    log.Fatal("Pairing request failed:", err)
}

if response.Accepted {
    fmt.Println("Pairing successful!")
    fmt.Println("Verification code:", response.PairingCode)
    
    // The same code should be displayed on both devices
    // Users can visually verify they match for enhanced security
} else {
    fmt.Println("Pairing rejected:", response.ErrorMessage)
}

// After pairing is done on Device A
syncManager.DisablePairing()
```

## Protocol Details

### Pairing Protocol

The pairing protocol enables secure device-to-device authentication:

- **Protocol Path**: `/clipman/1.0.0/pairing`
- **Security Model**:
  - Manual approval of pairing requests
  - Verification codes derived from both peer IDs
  - End-to-end encryption via libp2p
  - Persistent storage of paired devices
- **Pairing Flow**:
  1. Device A enters pairing mode and gets an address to share
  2. Device B connects to Device A using the address
  3. Device A prompts for confirmation
  4. Both devices display a verification code for visual confirmation
  5. Both devices store each other's information permanently

### Clipboard Protocol

The clipboard protocol allows sharing clipboard content between peers:

- **Protocol Path**: `/clipman/1.0.0/clipboard`
- **Content Types**: Text, Image, Files, HTML, and more
- **Metadata**: Content type, size, timestamp, source device
- **Delivery**: Direct to peer for 1:1 or via pubsub for groups

### File Transfer Protocol

The file transfer protocol enables transferring larger files:

- **Protocol Path**: `/clipman/1.0.0/file`
- **Features**: 
  - Resume support
  - Progress tracking
  - Integrity verification
  - Selective sync

## Configuration

The sync functionality is configured through a comprehensive configuration structure:

```go
// SyncConfig holds all sync-related configuration
type SyncConfig struct {
    // Core Sync Settings
    Enabled           bool     `json:"enable_sync"`
    SyncOverInternet  bool     `json:"sync_over_internet"`
    UseRelayNodes     bool     `json:"use_relay_nodes"`
    ListenPort        int      `json:"listen_port"`
    PeerIdentity      string   `json:"peer_identity"`
    DiscoveryMethod   string   `json:"discovery_method"` // "mdns", "dht", "manual", or "paired"
    DHTBootstrapPeers []string `json:"dht_bootstrap_peers"` // Bootstrap peers for DHT
    
    // Device Information
    DeviceName        string   `json:"device_name"`      // Human-readable device name
    DeviceType        string   `json:"device_type"`      // Type of device (desktop, mobile, etc.)
    
    // Peer Persistence
    PersistDiscoveredPeers bool   `json:"persist_discovered_peers"` // Whether to save discovered peers to disk
    DiscoveredPeersPath    string `json:"discovered_peers_path"`    // Path to store discovered peers
    AutoReconnectToPeers   bool   `json:"auto_reconnect_to_peers"`  // Whether to auto-reconnect to known peers
    MaxStoredPeers         int    `json:"max_stored_peers"`         // Maximum number of peers to store
    
    // Pairing
    PairingEnabled      bool     `json:"pairing_enabled"`       // Whether pairing functionality is enabled
    PairingTimeout      int      `json:"pairing_timeout"`       // Seconds before pairing mode times out (0 for no timeout)
    RequireVerification bool     `json:"require_verification"`  // Whether verification code must be confirmed
    
    // Clipboard Sync Options
    ClipboardTypes         []string `json:"clipboard_types"`         // "text", "image", "files"
    AutoCopyFromPeers      bool     `json:"auto_copy_from_peers"`
    MaxClipboardSizeKB     int      `json:"max_clipboard_size_kb"`
    ClipboardHistorySize   int      `json:"clipboard_history_size"`
    ClipboardBlacklistApps []string `json:"clipboard_blacklist_apps"`
    
    // File Transfer Options
    EnableFileSharing       bool     `json:"enable_file_sharing"`
    RequireFileConfirmation bool     `json:"require_file_confirmation"`
    DefaultDownloadFolder   string   `json:"default_download_folder"`
    AutoAcceptFromPeers     []string `json:"auto_accept_from_peers"`
    MaxFileSizeMB           int      `json:"max_file_size_mb"`
    
    // Privacy & Security
    AllowOnlyKnownPeers bool     `json:"allow_only_known_peers"`
    TrustedPeers        []string `json:"trusted_peers"`
    RequireApprovalPin  bool     `json:"require_approval_pin"`
    LogPeerActivity     bool     `json:"log_peer_activity"`
    
    // Developer & Debug Options
    DebugLogging              bool `json:"debug_logging"`
    ShowPeerDebugInfo         bool `json:"show_peer_debug_info"`
    DisableMultiplexing       bool `json:"disable_multiplexing"`
    ForceDirectConnectionOnly bool `json:"force_direct_connection_only"`
    
    // DHT Discovery Options
    DHTPersistentStorage bool   `json:"dht_persistent_storage"` // Whether to store DHT data on disk
    DHTStoragePath       string `json:"dht_storage_path"`       // Path to store DHT data
    DHTServerMode        bool   `json:"dht_server_mode"`        // Whether to run DHT in server mode
}
```

Configuration in JSON format:

```json
{
  "sync": {
    "enable_sync": true,
    "sync_over_internet": true,
    "use_relay_nodes": true,
    "listen_port": 0,
    "discovery_method": "paired",
    "dht_bootstrap_peers": [],
    
    "device_name": "My Laptop",
    "device_type": "desktop",
    
    "persist_discovered_peers": true,
    "discovered_peers_path": "~/.clipman/peers.json",
    "auto_reconnect_to_peers": true,
    "max_stored_peers": 100,
    
    "pairing_enabled": true,
    "pairing_timeout": 300,
    "require_verification": true,
    
    "clipboard_types": ["text", "image"],
    "auto_copy_from_peers": true,
    "max_clipboard_size_kb": 512,
    "clipboard_history_size": 50,
    "clipboard_blacklist_apps": [],
    
    "enable_file_sharing": true,
    "require_file_confirmation": true,
    "default_download_folder": "~/Downloads/Clipman",
    "auto_accept_from_peers": [],
    "max_file_size_mb": 100,
    
    "allow_only_known_peers": true,
    "trusted_peers": [],
    "require_approval_pin": false,
    "log_peer_activity": true,
    
    "debug_logging": false,
    "show_peer_debug_info": false,
    "disable_multiplexing": false,
    "force_direct_connection_only": false,
    
    "dht_persistent_storage": false,
    "dht_storage_path": "~/.clipman/dht",
    "dht_server_mode": false
  }
}
```

## Discovery Mechanisms

Clipman supports four complementary discovery mechanisms:

### 1. Secure Pairing (Recommended)

The most secure option for connecting devices:

- Manual, user-initiated pairing process
- Verification codes to confirm the correct devices are being paired
- Persistent connections between paired devices
- Highest security and reliability
- Implementation in `PairingManager` class

### 2. Local Discovery (mDNS)

mDNS discovery automatically finds peers on the local network:

- Zero configuration required
- Low latency for local connections
- Automatic discovery without manual setup
- Implementation in `MDNSDiscovery` class

### 3. Wide-area Discovery (DHT)

DHT discovery finds peers across the internet using the Kademlia Distributed Hash Table:

- Discover peers outside your local network
- Bootstrap from known peers
- Uses rendezvous points for application-specific discovery
- Persistent peer routing
- Customizable bootstrap peers
- Optional server mode to improve network health
- Implementation in `DHTDiscovery` and `RendezvousDHT` classes

### 4. Manual Discovery

Manual discovery allows direct connection to known peers:

- Add peers by their multiaddress
- Useful for known/fixed peers
- Connect across network boundaries
- Implementation in `ManualDiscovery` class

## Security Model

Clipman's synchronization has multiple layers of security:

1. **Transport Security**: libp2p provides encryption for all communications using the Noise protocol
2. **Identity**: Each device has a unique peer ID derived from a cryptographic key pair
3. **Pairing**: Manual confirmation for all device-to-device connections
4. **Verification**: Visual verification codes ensure correct device pairing
5. **Trust Levels**: Different trust for paired vs. discovered peers
6. **Persistence**: Paired devices are stored for future reconnection
7. **Control**: Users can disable discovery methods or limit to only paired devices

## Peer Persistence

Clipman supports persisting discovered peers between sessions:

- Saves peer information to disk
- Automatically reconnects to previously discovered peers
- Configurable storage path
- Maximum peer storage limit
- Peer aging/pruning mechanism

## PubSub Groups

Groups in Clipman are implemented using libp2p's GossipSub:

- Topic-based messaging
- Message signing and verification
- Efficient message propagation
- Message deduplication

## Future Enhancements

Planned enhancements include:

1. **Multiple Device Profiles**: Different sync profiles for different devices or use cases
2. **Conflict Resolution**: Smart handling of concurrent edits
3. **Bandwidth Controls**: Limiting sync based on network conditions
4. **Search**: Searching across synced content
5. **End-to-End Encryption**: Additional encryption layer for content
6. **History and Versioning**: Content history and version management
7. **Advanced DHT**: Custom validators and providers
8. **NAT Traversal Improvements**: Better connectivity across NATs
9. **Offline Mode**: Queue changes when offline for later sync
10. **Selective Sync**: Choose what content to sync between specific peers

## License

Same as the main project.