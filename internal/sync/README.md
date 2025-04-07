# Clipman Sync Package

This package provides clipboard synchronization capabilities for Clipman, allowing content to be synced across multiple devices.

## Architecture

The sync package follows a layered architecture with a plugin-based system for protocols:

```
                           ┌───────────────────┐
                           │    Application    │
                           └─────────┬─────────┘
                                     │
                                     ▼
                           ┌───────────────────┐
                           │    SyncManager    │
                           └─────────┬─────────┘
                                     │
                        ┌────────────┴────────────┐
                        │                         │
              ┌─────────▼─────────┐     ┌─────────▼─────────┐
              │      Protocol     │     │     Discovery     │
              └─────────┬─────────┘     └─────────┬─────────┘
                        │                         │
         ┌──────────────┴──────────────┐         │
         │                             │         │
┌────────▼───────┐           ┌─────────▼────────┐│
│ Protocol Impl. │           │ Protocol Impl.   ││
│    (MQTT)      │           │     (P2P)        ││
└────────────────┘           └──────────────────┘┘
```

### Key Components

1. **SyncManager**: The primary entry point and orchestrator for synchronization.
2. **Protocol Interface**: Abstracts communication protocols (MQTT, P2P).
3. **Discovery**: Handles peer discovery and announcements.
4. **Message**: Represents the messages exchanged between peers.
5. **Security**: Handles encryption, authentication, and integrity.

## Decentralized Factory Pattern

The sync package uses a decentralized factory pattern for protocol implementations:

1. Each protocol implementation (e.g., MQTT, P2P) has its own factory
2. Factories register themselves with a central registry in `protocol/protocol.go`
3. The `Manager` uses this registry to retrieve the appropriate factory

This design allows:
- Modular protocol implementations that can be added/removed easily
- Automatic registration of protocols via `init()`
- Extension of the system without modifying core code

## Directory Structure

```
internal/sync/
├── sync.go             # Main package exports and factory functions
├── interfaces.go       # Core interfaces (SyncManager, Protocol, Discovery)
├── manager.go          # SyncManager implementation
├── config.go           # Sync-specific configuration handling
├── discovery/
│   ├── discovery.go    # Interface definitions for peer discovery
│   ├── mdns.go         # mDNS implementation for local network discovery
│   └── mqtt.go         # MQTT-based discovery service
├── protocol/
│   ├── protocol.go     # Common protocol interface and utilities
│   ├── mqtt/
│   │   ├── client.go   # MQTT protocol client implementation
│   │   ├── factory.go  # MQTT factory for protocol registry
│   │   └── message.go  # MQTT message formats
│   └── p2p/
│       ├── client.go   # P2P protocol client implementation
│       ├── factory.go  # P2P factory for protocol registry
│       ├── nat.go      # NAT traversal utilities
│       └── message.go  # P2P message formats
├── transport/
│   ├── transport.go    # Transport layer interfaces
│   ├── direct.go       # Direct socket-based transport
│   └── relay.go        # Relay-based transport for NAT traversal
└── security/
    ├── security.go     # Security interfaces
    ├── encryption.go   # Content encryption utilities
    └── auth.go         # Authentication mechanisms
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
    syncManager, err := sync.NewSyncManager(cfg, logger)
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

### Adding a New Protocol Implementation

1. Create a new directory under `protocol/` for your protocol
2. Implement the required interfaces
3. Create a factory that registers itself:

```go
package newprotocol

import "github.com/berrythewa/clipman-daemon/internal/sync/protocol"

const (
    ProtocolName = "newprotocol"
)

// Factory is the protocol factory
type Factory struct{}

// NewFactory creates a new factory
func NewFactory() *Factory {
    return &Factory{}
}

// init registers the protocol factory
func init() {
    protocol.RegisterProtocolFactory(ProtocolName, NewFactory())
}
```

## Configuration

The sync functionality is configured through the main config.json:

```json
{
  "sync": {
    "enabled": true,
    "protocol": "mqtt",
    "url": "mqtt://broker.example.com:1883",
    "username": "username",
    "password": "password",
    "default_group": "default",
    "mode": "centralized",
    "discoverable": true,
    "client_id": "device123"
  }
}
```

## Protocol Details

### MQTT Protocol

The MQTT protocol implementation provides:
- Reliable clipboard content synchronization using MQTT brokers
- Topic structure: `clipman/{group}/{content|control}/{message_type}`
- Support for private groups with access control
- Automatic reconnection and message buffering

### P2P Protocol (Planned)

The P2P protocol implementation will provide:
- Direct device-to-device communication
- NAT traversal for connecting across networks
- Automatic peer discovery using mDNS
- End-to-end encryption for all traffic

## License

Same as main project. 