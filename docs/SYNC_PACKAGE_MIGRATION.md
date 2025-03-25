# Migration Plan: `broker` to `sync` Package

This document outlines the plan for renaming the `broker` package to `sync` and expanding its functionality to support the enhanced MQTT implementation.

## Overview

The current `broker` package is primarily focused on MQTT client connectivity. We'll rename it to `sync` to better reflect its expanded role in providing complete clipboard synchronization functionality across devices.

## Migration Steps

### 1. Package Structure Changes

#### Directory Restructuring
```
internal/
├── broker/              ← Rename to sync/
│   └── mqtt_client.go   ← Will become sync/mqtt_client.go
└── sync/                ← New package
    ├── mqtt_client.go   ← Migrated from broker
    ├── group.go         ← New file for sync groups
    ├── filter.go        ← New file for content filtering
    ├── encryption.go    ← New file for E2E encryption
    ├── discovery.go     ← New file for device discovery
    └── commands.go      ← New file for enhanced commands
```

#### Interface and Type Changes
- Rename `MQTTClientInterface` to `SyncClient` or similar
- Retain backward compatibility with wrapper or adapter types
- Expand interfaces to include new functionality

### 2. Code Migration Process

1. **Create New Package Structure**
   - Create the new `internal/sync` directory
   - Keep the old `internal/broker` temporarily for backward compatibility

2. **Incrementally Migrate Files**
   - Copy `mqtt_client.go` to `sync/mqtt_client.go`
   - Update package declaration from `package broker` to `package sync`
   - Update imports in the new file

3. **Update References**
   - Identify all references to the `broker` package using grep or similar tools
   - Update imports from `"github.com/berrythewa/clipman-daemon/internal/broker"` to `"github.com/berrythewa/clipman-daemon/internal/sync"`
   - Update type references from `broker.MQTTClient` to `sync.MQTTClient`

4. **Create Temporary Compatibility Layer**
   - Create a bridge in the old `broker` package that forwards to the new `sync` package
   - This allows for gradual migration while maintaining functionality

### 3. Interface Enhancements

Once the migration is complete, expand the interfaces to support new functionality:

```go
// SyncClient is the main interface for synchronization functionality
type SyncClient interface {
    // Original MQTT functionality
    PublishContent(content *types.ClipboardContent) error
    PublishCache(cache *types.CacheMessage) error
    SubscribeToCommands() error
    RegisterCommandHandler(commandType string, handler func([]byte) error)
    IsConnected() bool
    Disconnect() error
    
    // New functionality
    JoinGroup(groupID string) error
    LeaveGroup(groupID string) error
    ListGroups() ([]string, error)
    SetContentFilter(filter *SyncFilter) error
    EnableEncryption(options *EncryptionOptions) error
    // ...and more
}
```

### 4. Modified Files

The following files will need to be updated to reference the new package:

```
internal/clipboard/monitor.go          - Update imports and type references
internal/config/config.go              - Add new sync-related configuration options
internal/cli/root.go                   - Update broker client initialization
internal/cli/cmd/run.go                - Update broker client initialization
internal/storage/boltdb.go             - Update mqtt client references
internal/mocks/mock_broker.go          - Migrate to mock_sync.go
```

### 5. New Functionality Implementation

After the migration, implement the new functionality outlined in TODO.md:

1. Topic structure redesign
2. Content filtering
3. Security enhancements
4. Dynamic discovery
5. Enhanced command system

## Testing Strategy

1. **Unit Tests**
   - Create extensive tests for the new `sync` package
   - Ensure all original functionality is preserved
   - Test new features thoroughly

2. **Integration Tests**
   - Test synchronization between multiple instances
   - Verify group functionality
   - Test content filtering
   - Validate security measures

3. **Migration Validation**
   - Create tests that verify the original API still works through the compatibility layer
   - Check for any regressions in existing functionality

## Documentation Updates

1. **Update API Documentation**
   - Document the new `sync` package and its capabilities
   - Provide migration examples for existing users

2. **Update User Documentation**
   - Create new docs for sync groups, filtering, and security features
   - Update existing MQTT documentation to reference new package

## Rollout Plan

1. **Phase 1: Infrastructure**
   - Implement basic package renaming
   - Create compatibility layer
   - Update all references

2. **Phase 2: Core Features**
   - Implement sync groups
   - Add content filtering
   - Standardize command structure

3. **Phase 3: Advanced Features**
   - Add security enhancements
   - Implement device discovery
   - Add user experience improvements

## Backward Compatibility

During development, maintain backward compatibility by:

1. Keeping the original interfaces and methods
2. Adding new methods rather than changing existing ones
3. Using the compatibility layer to bridge old code to new code
4. Providing clear upgrade paths in documentation 