# Clipman Sync Package

This package provides clipboard synchronization capabilities for Clipman, allowing content to be synced across multiple devices.

## Architecture

The sync package follows a layered architecture with a plugin-based system for protocols and discovery mechanisms:

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
         ┌──────────────┴──────────────┐    ┌────┴───────────────┐
         │                             │    │                     │
┌────────▼───────┐           ┌─────────▼────┴───┐   ┌─────────────▼───┐
│ Protocol Impl. │           │ Protocol Impl.    │   │ Discovery Impl. │
│    (MQTT)      │           │     (P2P)         │   │    (mDNS)       │
└────────────────┘           └───────────────────┘   └─────────────────┘
```

### Key Components

1. **SyncManager**: The primary entry point and orchestrator for synchronization. Implements the `SyncManager` interface.
2. **Protocol**: Abstracts communication protocols (MQTT, P2P). Implements the `Protocol` interface.
3. **Discovery**: Handles peer discovery and announcements. Implements the `Discovery` interface.
4. **Message**: Represents the messages exchanged between peers. Implements the `Message` interface.

## Decentralized Factory Pattern

The sync package uses a decentralized factory pattern for both protocol and discovery implementations:

1. Each protocol/discovery implementation has its own factory that implements the respective factory interface
2. Factories register themselves with a central registry in their respective packages
3. The `Manager` uses these registries to retrieve the appropriate factory based on configuration

This design allows:
- Modular implementations that can be added/removed easily
- Automatic registration of components via `init()`
- Extension of the system without modifying core code

### Registration Flow

```
┌────────────────┐    registers     ┌──────────────────┐
│ Protocol Impl. ├───────────────►  │ Protocol Registry │
└────────────────┘                  └──────────────────┘
                                              ▲
                                              │ looks up
                                              │
┌────────────────┐    requests      ┌────────┴─────────┐
│   Application  ├───────────────►  │    SyncManager   │
└────────────────┘                  └──────────────────┘
```

## Current Implementation

Currently, the following components are implemented:

- **Core interfaces**: `SyncManager`, `Protocol`, `Discovery`, and `Message`
- **Manager**: The main implementation of the `SyncManager` interface
- **MQTT Protocol**: Implementation of the `Protocol` interface using MQTT
- **mDNS Discovery**: Implementation of the `Discovery` interface using mDNS

## Directory Structure

```
internal/sync/
├── sync.go             # Main package exports and factory functions
├── interfaces.go       # Core interfaces (SyncManager, Protocol, Discovery)
├── manager.go          # SyncManager implementation
├── config.go           # Sync-specific configuration handling
├── discovery/
│   ├── discovery.go    # Common discovery interfaces and utilities
│   └── mdns/
│       └── mdns.go     # mDNS implementation for local network discovery
└── protocol/
    ├── protocol.go     # Common protocol interface and utilities
    └── mqtt/
        ├── client.go   # MQTT protocol client implementation
        ├── factory.go  # MQTT factory for protocol registry
        └── message.go  # MQTT message formats
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
2. Implement the required interfaces:
   - `protocol.Client` - Client implementation
   - `protocol.Factory` - Factory for creating clients
   - `protocol.Message` - Message format specific to your protocol
3. Create a factory that registers itself in `init()`:

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

### Adding a New Discovery Implementation

Similar to adding a protocol implementation:

1. Create a new directory under `discovery/` for your discovery mechanism
2. Implement the required interfaces
3. Register your factory in `init()`

## Configuration

The sync functionality is configured through the centralized configuration:

```json
{
  "sync": {
    "enabled": true,
    "protocol": "mqtt",
    "url": "mqtt://broker.example.com:1883",
    "username": "username",
    "password": "password",
    "default_group": "default",
    "enable_discovery": true,
    "discovery_method": "mdns",
    "client_id": "device123"
  }
}
```

## Protocol Details

### MQTT Protocol

The MQTT protocol implementation provides:
- Reliable clipboard content synchronization using MQTT brokers
- Topic structure: `clipman/{group}/{content|control}/{message_type}`
- Support for groups with different access levels
- Automatic reconnection with configurable retry logic
- Well-structured messages with metadata

### mDNS Discovery

The mDNS discovery implementation provides:
- Local network peer discovery using Multicast DNS
- Automatic announcement of device presence
- Device information sharing (capabilities, groups, etc.)
- Regular announcements to maintain peer lists

## Future Enhancements

Planned enhancements include:

1. **P2P Protocol**: Direct device-to-device communication without central servers
2. **End-to-End Encryption**: Security layer for message encryption 
3. **File Transfer**: Large file transfer capabilities
4. **DHT Discovery**: Distributed Hash Table based discovery for P2P networks
5. **Transport Abstraction**: Lower-level network transport abstraction
6. **Conflict Resolution**: Improved handling of concurrent edits

## License

Same as the main project.