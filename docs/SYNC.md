# Clipman Synchronization Logic

This document explains how clipboard content synchronization works in Clipman, with a focus on the device selection model and paginated history access.

## Overview

Clipman provides a sophisticated device-to-device synchronization system that allows users to securely share clipboard content across multiple devices. The system is built on the following principles:

1. **Secure Pairing**: Only explicitly paired devices can exchange clipboard content
2. **Selective Sync**: Users can choose which specific devices to sync with
3. **Paginated History**: Access to remote device clipboard history with efficient pagination
4. **Privacy Control**: Full control over what content is shared and with which devices

## Architecture

The synchronization system consists of several interconnected components:

```
┌─────────────────┐     ┌──────────────────┐     ┌──────────────────┐
│                 │     │                  │     │                  │
│  User Interface │───▶│  Sync Manager    │───▶│  Pairing Manager │
│                 │     │                  │     │                  │
└─────────────────┘     └──────────────────┘     └──────────────────┘
                               │   ▲                      │
                               │   │                      │
                               ▼   │                      ▼
                        ┌──────────────────┐     ┌──────────────────┐
                        │                  │     │                  │
                        │ Content Exchanger│     │Discovery Manager │
                        │                  │     │                  │
                        └──────────────────┘     └──────────────────┘
                               │   ▲                      │
                               │   │                      │
                               ▼   │                      ▼
                        ┌──────────────────┐     ┌──────────────────┐
                        │                  │     │                  │
                        │ Protocol Manager │     │  libp2p Network  │
                        │                  │     │                  │
                        └──────────────────┘     └──────────────────┘
```

## Device Selection Model

Clipman implements a device-centric model for accessing clipboard content:

1. **Device List**: The user sees a list of all paired devices
2. **Device Selection**: The user selects a specific device to view its clipboard history
3. **Content Retrieval**: Content is fetched from the selected device on demand
4. **Pagination**: Results are paginated for efficient loading and browsing

This approach differs from a traditional "sync everything" model, giving users more control and reducing unnecessary data transfer.

## Pagination Logic

When browsing a remote device's clipboard history, Clipman uses efficient pagination:

1. **Page Size**: Each request fetches a configurable number of items (default: 20)
2. **Cursor-based**: Pagination uses cursors rather than offset to ensure consistency
3. **On-demand Loading**: Content is loaded only when requested, not preemptively
4. **Direction Control**: Support for both forward and backward pagination

### Pagination Implementation

```go
type HistoryRequest struct {
    DeviceID  string    // ID of the target device
    Cursor    string    // Pagination cursor
    PageSize  int       // Number of items to retrieve (default: 20)
    Direction string    // "forward" or "backward"
    Filter    *Filter   // Optional content filters
}

type HistoryResponse struct {
    Items       []*ClipboardContent // Array of clipboard items
    NextCursor  string              // Cursor for the next page, if available
    PrevCursor  string              // Cursor for the previous page, if available
    TotalItems  int                 // Total number of items matching the filter
    CurrentPage int                 // Current page number
}
```

## Content Exchange Protocol

The protocol for exchanging clipboard content follows these steps:

1. **Request**: Device A sends a history request to Device B
2. **Authentication**: Device B verifies that Device A is paired
3. **Authorization**: Device B checks if Device A is authorized to access its history
4. **Filtering**: Device B applies any filters specified in the request
5. **Pagination**: Device B selects the appropriate page of results
6. **Response**: Device B sends the results back to Device A
7. **Display**: Device A displays the content to the user

### Message Sequence

```
┌────────────┐                      ┌────────────┐
│  Device A  │                      │  Device B  │
└──────┬─────┘                      └──────┬─────┘
       │                                   │
       │  1. HistoryRequest                │
       │─────────────────────────────────>│
       │                                   │
       │                                   │ 2. Authenticate
       │                                   │ 3. Authorize
       │                                   │ 4. Apply filters
       │                                   │ 5. Paginate
       │                                   │
       │  6. HistoryResponse               │
       │<─────────────────────────────────│
       │                                   │
       │  7. Display to user               │
       │                                   │
```

## User Experience

### Selecting a Device

Users can select a device to view its clipboard history:

```bash
# List all paired devices
clipman devices list

# View clipboard history from a specific device
clipman content --from-device "laptop-work"

# Apply pagination
clipman content --from-device "laptop-work" --page 2 --page-size 10
```

Through the UI, users see a list of paired devices and can click on any device to view its clipboard history, with pagination controls available for navigating through the history.

### Real-time Updates

When new content is copied on a remote device, notifications can be sent to other paired devices:

1. **Push Notifications**: Immediate notification when new content is available
2. **Pull Updates**: Regular polling for new content (configurable interval)
3. **Manual Refresh**: User-initiated refresh to get the latest content

## Filtering Options

When requesting content from a remote device, various filters can be applied:

1. **Content Type**: Filter by text, image, URL, etc.
2. **Time Range**: Get content created within a specific time period
3. **Size Limits**: Filter by content size (min/max)
4. **Search Text**: Filter text content by search terms
5. **Tags/Labels**: Filter by custom tags (if supported)

## Synchronization Modes

Clipman supports multiple synchronization modes:

1. **On-demand**: Content is only fetched when explicitly requested (default)
2. **Background**: Content is synced in the background at regular intervals
3. **Real-time**: Content is immediately synced when changes occur
4. **Selective**: Only specific content types are synchronized

These modes can be configured globally or per-device.

## Configuration Options

The synchronization behavior can be customized through several configuration options:

```json
{
  "sync": {
    "enable_sync": true,
    "sync_mode": "on-demand",        // "on-demand", "background", "real-time"
    "page_size": 20,                 // Default number of items per page
    "background_sync_interval": 300, // Seconds between background syncs
    "max_content_size_kb": 1024,     // Maximum size of content to sync
    "preferred_devices": [           // Devices to prioritize for syncing
      "laptop-work",
      "phone-personal"
    ],
    "content_types": [               // Types of content to sync
      "text",
      "url",
      "image"
    ]
  }
}
```

## User Interface Considerations

The UI for browsing remote device content should include:

1. **Device Selector**: Clear way to select which device to view
2. **Pagination Controls**: Next/previous page buttons and page indicators
3. **Loading Indicators**: Visual feedback during content loading
4. **Error Handling**: Clear messaging when content retrieval fails
5. **Refresh Button**: Manual way to refresh content
6. **Filters**: UI for setting content filters
7. **Sorting Options**: Control over content ordering

## Security Considerations

To maintain security in the sync system:

1. **Authentication**: Every request is authenticated using the paired device credentials
2. **Authorization**: Devices can only access content they are authorized to see
3. **Encryption**: All content transfers are encrypted end-to-end
4. **Audit Logging**: All access to remote content is logged
5. **Rate Limiting**: Requests are rate-limited to prevent abuse
6. **Timeout Handling**: Connections timeout after periods of inactivity

## Implementation Notes

### Cursor Implementation

Cursors are implemented as base64-encoded strings containing:

```json
{
  "device_id": "laptop-work",
  "timestamp": "2023-04-25T10:15:30Z",
  "item_id": "abcd1234",
  "direction": "forward"
}
```

This ensures consistent pagination even if items are added or removed between requests.

### Caching Strategy

To improve performance, Clipman implements a multi-level caching strategy:

1. **Local Cache**: Recently accessed remote content is cached locally
2. **Cache Invalidation**: Cache is invalidated when changes are detected
3. **Cache TTL**: Content expires from cache after a configurable time
4. **Prefetching**: Next page can be prefetched when browsing
5. **Background Updates**: Cache can be updated in the background

## Error Handling

The sync system handles various error conditions:

1. **Device Offline**: Graceful handling when target device is not available
2. **Network Errors**: Retry logic with exponential backoff
3. **Permission Errors**: Clear messaging when access is denied
4. **Timeout Errors**: Recovery when requests take too long
5. **Invalid Cursors**: Fallback to first page when pagination cursors are invalid

## Future Enhancements

Planned improvements to the synchronization system:

1. **Smart Sync**: AI-driven decisions about what content to sync based on usage patterns
2. **Conflict Resolution**: Better handling of clipboard conflicts between devices
3. **Favorite Devices**: Mark certain devices as favorites for quicker access
4. **Device Groups**: Create logical groupings of devices for better organization
5. **Search Across Devices**: Unified search across all paired devices
6. **Offline Mode**: Better support for offline operation with sync when reconnected
7. **Historical View**: Timeline view of clipboard history across all devices

## API Reference

### Device Operations

```go
// Get list of paired devices
GetPairedDevices() []PairedDevice

// Select a device for content retrieval
SelectDevice(deviceID string) error

// Check if device is online
IsDeviceOnline(deviceID string) bool
```

### Content Operations

```go
// Get paginated content from a selected device
GetRemoteContent(request HistoryRequest) (HistoryResponse, error)

// Copy remote content to local clipboard
CopyToLocalClipboard(contentID string) error

// Send local content to remote device
SendToRemoteDevice(contentID string, deviceID string) error
```

## Troubleshooting

Common issues and solutions:

1. **Content Not Appearing**: Check if device is online and paired properly
2. **Slow Loading**: Reduce page size or check network connection
3. **Missing History**: Verify that target device has history retention enabled
4. **Access Denied**: Re-pair devices if authorization issues occur
5. **Incorrect Content**: Ensure content types are supported on both devices 