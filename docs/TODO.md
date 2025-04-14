# Clipman MQTT Implementation: TODO

This document outlines the tasks for enhancing Clipman's MQTT-based synchronization system.

## 1. Topic Structure Redesign

- [ ] Implement "sync groups" concept in config system
- [ ] Create new hierarchical topic structure:
  ```
  clipman/group/{groupID}/{messageType}/{deviceID}
  ```
- [ ] Modify topic generation functions to use group-based structure
- [ ] Add group management commands
- [ ] Implement wildcard subscriptions for group monitoring
- [ ] Add support for joining multiple groups simultaneously

## 2. Content Filtering System

- [ ] Create `SyncFilter` configuration structure
- [ ] Implement content type filtering (text, image, file, etc.)
- [ ] Add size-based filtering with configurable limits
- [ ] Implement pattern-based filtering for text content
- [ ] Add path filtering for file/filepath types
- [ ] Implement on-demand content fetching protocol for large content
- [ ] Add CLI options and config settings for managing filters

## 3. Security Enhancements

- [ ] Implement end-to-end encryption using libsodium
- [ ] Create key management system for encryption
- [ ] Add key exchange protocol for group members
- [x] Implement device registration and authorization process
- [x] Add challenge-response mechanism for device verification
- [ ] Implement content signatures for authenticity verification
- [ ] Add replay protection mechanisms
- [ ] Document security best practices
- [ ] Create security audit tools

## 4. Dynamic Discovery and Subscription

- [x] Implement device presence announcements
- [x] Create heartbeat system for online/offline detection
- [x] Add capability advertisement in presence messages
- [ ] Develop group discovery protocol
- [x] Implement invitation mechanism for private groups (secure pairing)
- [ ] Add support for temporary/session-based groups
- [ ] Create selective subscription system
- [ ] Implement subscription rotation for load balancing

## 5. Enhanced Command System

- [ ] Define standardized command structure with versioning
- [ ] Implement command categories (admin, content, system)
- [ ] Add acknowledgment and response protocols
- [ ] Implement richer command set:
  - [ ] Remote clipboard operations (request, clear, search)
  - [ ] Group management commands (join, leave, invite)
  - [ ] System commands (status, diagnostics, logs)
  - [ ] Content management commands (purge, archive, tag)
- [ ] Add command authorization levels
- [ ] Implement command validation and rate limiting
- [ ] Add audit logging for sensitive commands

## 6. Performance Optimizations

- [ ] Enable and optimize content compression
- [ ] Implement adaptive compression based on content type
- [ ] Add deduplication for repetitive content
- [ ] Implement configurable rate limiting
- [ ] Create priority queues for different content types
- [ ] Add background synchronization for large content
- [ ] Implement delta synchronization for incremental updates
- [ ] Develop smart cache reconciliation algorithms

## 7. User Experience Enhancements

- [x] Add visual indicators for synchronization status
- [ ] Implement detailed sync logs and statistics
- [ ] Create notification system for sync events
- [x] Add user interface for controlling what gets synchronized
- [ ] Implement manual sync triggers
- [ ] Create privacy mode to temporarily disable synchronization
- [ ] Build user-facing conflict resolution for ambiguous cases
- [ ] Develop automatic merging where possible
- [ ] Maintain and display synchronization history

## 8. Device Pairing and Management

- [x] Implement secure device pairing protocol
- [x] Create pairing command interface in CLI
- [x] Add verification code generation for visual confirmation
- [x] Implement paired device persistence and management
- [x] Create pairing-based discovery method
- [x] Add user documentation for the pairing process
- [x] Implement pairing timeout and auto-accept options
- [ ] Add QR code generation for mobile device pairing

## Implementation Phases

### Phase 1: Core Architecture (2-3 weeks)
- Topic structure redesign
- Basic filtering system
- Command schema standardization
- ✅ Secure device pairing

### Phase 2: Security & Performance (3-4 weeks)
- End-to-end encryption
- Compression enablement
- Bandwidth management
- ✅ Paired device discovery

### Phase 3: Advanced Features (4-5 weeks)
- Dynamic discovery
- Enhanced command set
- User experience improvements

### Phase 4: Refinement & Documentation (2-3 weeks)
- Testing and optimization
- Comprehensive documentation
- Example configurations and setup guides

## File Changes

### New Files
- `internal/sync/group.go` - Sync group implementation
- `internal/sync/filter.go` - Content filtering system
- `internal/sync/encryption.go` - E2E encryption implementation
- ✅ `internal/sync/discovery.go` - Device discovery protocol
- ✅ `internal/sync/pairing.go` - Secure device pairing protocol
- `internal/sync/commands.go` - Enhanced command structure
- `docs/MQTT_GROUPS.md` - Documentation for sync groups
- ✅ `docs/PAIRING.md` - Documentation for secure device pairing

### Modified Files
- `internal/broker/mqtt_client.go` - Update for new topic structure and features
- `internal/config/config.go` - Add new configuration options
- `internal/types/shared.go` - Enhance message types 
- `internal/clipboard/monitor.go` - Update content publishing logic
- ✅ `internal/cli/cmd/pair.go` - Add pairing command interface

## Development Guidelines

1. **Backward Compatibility**: Ensure changes maintain compatibility with older versions when possible
2. **Incremental Implementation**: Build features iteratively with tests for each component
3. **Security First**: Prioritize security in all design decisions
4. **Configuration**: Make all new features configurable
5. **Documentation**: Update docs alongside code changes

## Next Steps

1. Set up testing infrastructure for MQTT implementation
2. Create detailed technical specifications for each component
3. Begin implementing Phase 1 components
4. Schedule regular reviews of MQTT-related code 