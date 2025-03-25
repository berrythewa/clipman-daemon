# Clipman Caching System

This document provides a detailed explanation of the caching architecture in Clipman, including how clipboard data is stored, managed, and retrieved.

## Overview

Clipman implements a multi-level caching system that balances performance and persistence:

1. **Persistent Storage (BoltDB)**: Provides long-term storage of clipboard contents with configurable limits
2. **In-Memory History (Ring Buffer)**: Offers quick access to recently copied items

## Storage Implementation (BoltDB)

### Configuration

The BoltDB storage layer is configured through the `StorageConfig` struct with the following options:

- `DBPath`: Path to the database file (defaults to `{data_dir}/clipboard.db`)
- `MaxSize`: Maximum size of the clipboard history cache in bytes (default: 100MB)
- `KeepItems`: Number of items to keep when flushing the cache (default: 10)
- `DeviceID`: Device identifier used for synchronization
- `Logger`: Logger instance for storage operations
- `MQTTClient`: Optional MQTT client for synchronization

### Key Features

- **Atomic Counter**: Tracks the total size of stored data using an atomic counter for thread safety
- **Size-Based Auto-Flushing**: Automatically triggers cache flushing when total size exceeds the configured limit
- **Flexible Content Storage**: Stores clipboard content with timestamps, content types, and device information
- **Optimized Retrieval**: Supports retrieving latest content or filtered history based on multiple criteria

### Cache Management

#### Size Tracking

```go
// Increment the size counter atomically
atomic.AddInt64(&s.size, int64(len(content.Data)))
```

#### Auto-Flushing Logic

When saving new content, the storage layer checks if the total size exceeds the maximum:

```go
// Check if we need to flush the cache
if s.size > s.maxSize {
    s.logger.Info("Cache size exceeded threshold, flushing old content",
        "current_size", s.size,
        "max_size", s.maxSize)
    
    if err := s.flushOldestContent(s.keepItems); err != nil {
        s.logger.Error("Failed to flush oldest content", "error", err)
    }
}
```

#### Flushing Algorithm

The `flushOldestContent` method:

1. Retrieves all clipboard content keys and timestamps
2. Sorts them by timestamp (newest first)
3. Keeps the specified number of most recent items
4. Deletes the oldest items and updates the size counter

```go
// Algorithm pseudo-code:
1. Collect all content entries with their timestamps
2. Sort entries by timestamp (newest first)
3. Identify which items to keep (most recent n items)
4. Delete all other items
5. Update the size counter
6. Optionally publish deleted items via MQTT
```

## In-Memory History (Ring Buffer)

The in-memory history provides quick access to recently copied items without database queries.

### Implementation

Uses a `ring.Ring` structure from Go's container package to create a circular buffer:

```go
type ClipboardHistory struct {
    ring  *ring.Ring
    mutex sync.RWMutex
    size  int
}
```

### Key Features

- **Fixed Size**: Currently hardcoded to 100 items
- **Thread Safety**: Uses a mutex for concurrent access protection
- **Efficient Retrieval**: O(1) lookup for the most recent item
- **Circular Buffer**: Automatically overwrites oldest items when full

### Usage

```go
// Add an item to history
history.Add(newContent)

// Get the last n items
recentItems := history.GetLast(10)
```

## Content Processing Pipeline

Before being cached, clipboard content goes through a processing pipeline:

1. **Content Type Detection**: Identifies content type (text, URL, image, etc.)
2. **Filtering**: Applies filters to determine if content should be saved
   - Current default: Length filter (max 1000 characters)
3. **Transformation**: Modifies content before saving
   - Current default: Trims whitespace
4. **Compression**: (Currently disabled) Compresses content to save space

## Caching Flow

When new clipboard content is detected:

1. Platform-specific monitoring detects clipboard change
2. Content is processed through the pipeline
3. Content is added to the in-memory history ring buffer
4. Content is saved to persistent storage (BoltDB)
5. If configured, content is published to MQTT for synchronization
6. If storage size exceeds limit, oldest content is flushed

## Cache Management Commands

### Flush Command

The `flush` command allows manual flushing of the cache:

```bash
clipmand flush [--quiet]
```

It keeps a configurable number of most recent items (default: 10) and removes all older content.

### History Command

The `history` command retrieves cached items with various filtering options:

```bash
clipmand history [--limit n] [--type TYPE] [--reverse] [--since TIME] [--before TIME] [--min-size BYTES] [--max-size BYTES] [--json]
```

## Performance Considerations

- **Write Performance**: O(log n) for saving new content (BoltDB B+tree)
- **Read Performance**: 
  - O(1) for most recent items (in-memory history)
  - O(log n) for database access (BoltDB B+tree)
- **Memory Usage**: Fixed size in-memory buffer (100 items) plus BoltDB memory usage
- **Disk Usage**: Controlled by `max_size` configuration (default: 100MB)

## Optimization Opportunities

1. **Enabling Compression**: Implementing the commented-out compression code could reduce storage requirements
2. **Configurable In-Memory History Size**: Allow users to tune the ring buffer size for their needs
3. **Smarter Pruning Strategies**: Base flushing decisions on content age, size, and frequency of use
4. **Content Deduplication**: Detect and merge duplicate content to save space
5. **Type-Specific Optimizations**: Different storage strategies for different content types

## Debugging Cache Issues

If you encounter issues with the caching system, try:

1. Enable debug logging: `--log-level debug`
2. Check current cache size: `clipmand info`
3. Manually flush the cache: `clipmand flush`
4. Check the database file size: `ls -lh ~/.clipman/clipboard.db`
5. Reset the cache entirely: `rm ~/.clipman/clipboard.db` (caution: deletes all history) 