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
                                                          │
                                                    ┌─────▼─────┐
                                                    │  file.go  │
                                                    └───────────┘
```

### Key Components

1. **sync.go**: The primary entry point and orchestrator for synchronization. Provides high-level API for applications.
2. **node.go**: Core libp2p node setup, managing the host, identity, and connections.
3. **protocols.go**: Defines and manages custom protocols for clipboard and file synchronization.
4. **discovery.go**: Handles peer discovery using libp2p's built-in methods (mDNS and DHT).
5. **clipboard.go**: Implements clipboard content synchronization logic.
6. **file.go**: Implements file transfer functionality.
7. **types.go**: Common types and interfaces used throughout the package.

## libp2p Integration

The sync package leverages libp2p for all networking capabilities:

1. **Direct P2P Communication**: Content is transferred directly between peers without central servers
2. **Multiple Transport Support**: Uses libp2p's transport layer (TCP, QUIC, WebRTC)
3. **NAT Traversal**: Automatic NAT hole punching and relay support for connectivity
4. **Secure Communication**: End-to-end encryption with libp2p's secure transports
5. **PubSub**: Group messaging using libp2p's GossipSub
6. **DHT**: Distributed peer discovery for wide-area networking
7. **mDNS**: Local network discovery

## Directory Structure

```
internal/sync/
├── sync.go             # High-level sync manager API (entrypoint)
├── node.go             # Core libp2p node setup (host, discovery, routing)
├── protocols.go        # Define custom protocols (e.g., clipboard, file)
├── discovery.go        # mDNS / DHT discovery logic
├── clipboard.go        # Clipboard sync handler
├── file.go             # File transfer logic
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
    syncManager, err := sync.New(cfg, logger)
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
    syncManager.SetContentHandler(func(content *types.ClipboardContent, peer sync.PeerInfo) {
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
}
```

## Protocol Details

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

The sync functionality is configured through the centralized configuration:

```json
{
  "sync": {
    "enabled": true,
    "peer_id": "optional-persistent-peer-id",
    "libp2p": {
      "listen_addresses": [
        "/ip4/0.0.0.0/tcp/0",
        "/ip4/0.0.0.0/udp/0/quic"
      ],
      "bootstrap_peers": [],
      "enable_relay": true,
      "enable_nat": true,
      "dht": {
        "enabled": true,
        "bootstrap_peers": []
      },
      "mdns": {
        "enabled": true,
        "service_tag": "clipman"
      },
      "pubsub": {
        "enabled": true,
        "sign_messages": true
      }
    },
    "default_group": "default"
  }
}
```

## Discovery Mechanisms

### Local Discovery (mDNS)

The sync package uses libp2p's built-in mDNS discovery for finding peers on the local network:

- Automatic discovery without configuration
- Low latency for local connections
- Zero configuration required

### Wide-area Discovery (DHT)

For peers outside the local network, the sync package uses libp2p's Kademlia DHT:

- Find peers across the internet
- Bootstrap from known peers
- Persistent peer routing

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

## License

Same as the main project.