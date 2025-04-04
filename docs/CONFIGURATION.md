# Clipman Configuration Guide

This document provides a detailed breakdown of all available configuration options in Clipman and how to set them.

## Configuration Sources

Clipman follows a hierarchical configuration system with the following priority (highest to lowest):

1. **Command-line flags** - Override all other settings
2. **Environment variables** - Override config file settings
3. **Configuration file** - JSON format with default location based on platform
4. **Default values** - Built-in sensible defaults

## Configuration File Locations

The configuration file is automatically created at the platform-specific location if it doesn't exist:

- **Linux**: `$XDG_CONFIG_HOME/clipman/config.json` (typically `~/.config/clipman/config.json`)
- **macOS**: `~/Library/Application Support/com.berrythewa.clipman/config.json`
- **Windows**: `%APPDATA%\Clipman\config.json`

You can override this with the `CLIPMAN_CONFIG` environment variable or the `--config` flag.

## Data Directory Locations

By default, Clipman stores its database and other data files in these locations:

- **Linux**: `$XDG_DATA_HOME/clipman` (typically `~/.local/share/clipman`) or `~/.clipman`
- **macOS**: `~/Library/Application Support/Clipman`
- **Windows**: `%LOCALAPPDATA%\Clipman\Data`

You can override this with the `CLIPMAN_DATA_DIR` environment variable.

## Configuration Options

### General Settings

| Option | Config File Key | Environment Variable | Command Flag | Default | Description |
|--------|----------------|---------------------|------------|---------|-------------|
| Log Level | `log_level` | `CLIPMAN_LOG_LEVEL` | `--log-level` | `info` | Logging level (`debug`, `info`, `warn`, `error`) |
| Device ID | `device_id` | `CLIPMAN_DEVICE_ID` | `--device-id` | Auto-generated | Unique identifier for this device |
| Data Directory | `data_dir` | `CLIPMAN_DATA_DIR` | - | Platform-specific | Directory to store all application data |
| Polling Interval | `polling_interval` | - | - | `1s` | Interval for checking clipboard changes (on platforms without native events) |
| Config File | - | `CLIPMAN_CONFIG` | `--config` | Platform-specific | Path to configuration file |

### Storage Settings

| Option | Config File Key | Environment Variable | Command Flag | Default | Description |
|--------|----------------|---------------------|------------|---------|-------------|
| Database Path | `storage.db_path` | `CLIPMAN_STORAGE_PATH` | - | `{data_dir}/clipboard.db` | Path to the BoltDB database file |
| Maximum Size | `storage.max_size` | - | `--max-size` | 100MB | Maximum size of clipboard history in bytes |
| Keep Items | `storage.keep_items` | - | - | 10 | Number of items to keep when flushing cache |

### MQTT Broker Settings

| Option | Config File Key | Environment Variable | Command Flag | Default | Description |
|--------|----------------|---------------------|------------|---------|-------------|
| URL | `broker.url` | `CLIPMAN_BROKER_URL` | - | Empty | URL of the MQTT broker for syncing |
| Username | `broker.username` | `CLIPMAN_BROKER_USERNAME` | - | Empty | Username for broker authentication |
| Password | `broker.password` | `CLIPMAN_BROKER_PASSWORD` | - | Empty | Password for broker authentication |
| Disable Broker | - | - | `--no-broker` | false | Disable MQTT connection even if configured |

### History Options

| Option | Config File Key | Environment Variable | Command Flag | Default | Description |
|--------|----------------|---------------------|------------|---------|-------------|
| Limit | `history.limit` | - | `--limit` | 0 (no limit) | Maximum number of entries to retrieve |
| Since | `history.since` | - | `--since` | Empty | Retrieve entries since this time (RFC3339) |
| Before | `history.before` | - | `--before` | Empty | Retrieve entries before this time (RFC3339) |
| Content Type | `history.content_type` | - | `--type` | Empty (all types) | Filter by content type (`text`, `image`, `url`, etc.) |
| Reverse | `history.reverse` | - | `--reverse` | false | Reverse order (newest first when true) |
| Min Size | `history.min_size` | - | `--min-size` | 0 | Minimum content size in bytes |
| Max Size | `history.max_size` | - | `--max-size` | 0 | Maximum content size in bytes |

### Logging Settings

| Option | Config File Key | Environment Variable | Command Flag | Default | Description |
|--------|----------------|---------------------|------------|---------|-------------|
| Enable File Logging | `log.enable_file_logging` | `CLIPMAN_LOG_FILE_ENABLED` | `--no-file-log` | true | Enable logging to file |
| Max Log Size | `log.max_log_size` | - | - | 10MB | Maximum size of log files before rotation |
| Max Log Files | `log.max_log_files` | - | - | 5 | Maximum number of log files to keep |
| Log Format | `log.format` | - | - | `text` | Log format (`text` or `json`) |

## Command-Specific Flags

### Run Command

| Flag | Default | Description |
|------|---------|-------------|
| `--duration`, `-d` | 0 (indefinite) | Run for a specific duration (for testing) |
| `--detach` | false | Detach from terminal and run in background |

### History Command

| Flag | Default | Description |
|------|---------|-------------|
| `--json` | false | Output history in JSON format |
| `--dump-all` | false | Dump complete history without filters |

### Flush Command

| Flag | Default | Description |
|------|---------|-------------|
| `--quiet` | false | Don't display history before and after flush |

## Advanced Configuration

### Clipboard Monitoring Settings

| Option | Config File Key | Environment Variable | Command Flag | Default | Description |
|--------|----------------|---------------------|------------|---------|-------------|
| Stealth Mode | `stealth_mode` | `CLIPMAN_STEALTH_MODE` | `--no-stealth` | `true` | Minimize clipboard access notifications |
| Polling Interval | `polling_interval` | `CLIPMAN_POLLING_INTERVAL` | `--polling-interval` | `10000` | Base polling interval in milliseconds |

### Stealth Mode

Stealth mode is designed to minimize clipboard access notifications from desktop environments and is enabled by default. When enabled:

1. The daemon checks clipboard formats less frequently
2. The polling interval is significantly increased (10 seconds by default)
3. Clipboard content is only read when format changes are detected
4. Multiple checks are skipped to reduce the frequency of clipboard access

To disable stealth mode (not recommended for most users):

```bash
# Disable via command line
clipman --no-stealth

# Set a lower polling interval (ms) for more frequent checks
clipman --polling-interval 2000  # 2 seconds
```

Or modify your configuration file:

```json
{
  "stealth_mode": false,
  "polling_interval": 2000
}
```

**Note**: Disabling stealth mode will cause more frequent clipboard access, which may result in notifications from your desktop environment and potentially higher resource usage.

### Clipboard Monitoring

The clipboard monitoring functionality can be customized through the configuration:

- **Polling Interval**: Controls how frequently the daemon checks for clipboard changes on platforms without event-based notification.
- **Content Processing**: The `ContentProcessor` applies filters and transformers to clipboard content before saving:
  - Default length filter: 1000 characters (hardcoded)
  - Default transformer: Trims whitespace (hardcoded)

### In-Memory History

The `ClipboardHistory` struct maintains a circular buffer of recent clipboard items (last 100 items by default). This provides quick access to recent items without database queries.

### Caching System

Clipman implements a multi-level caching system:

1. **Persistent Storage (BoltDB)**:
   - Managed through the `BoltStorage` struct with configurable size limits
   - Automatic pruning when size exceeds the configured maximum
   - Content compression (currently disabled in code)
   - Configurable number of items to keep when flushing

2. **In-Memory History (Ring Buffer)**:
   - Fixed-size circular buffer for quick access to recent clipboard items
   - Thread-safe with mutex protection
   - Default size of 100 items (currently hardcoded)

### Platform-Specific Behaviors

- **Linux**: Uses polling with 500ms interval (hardcoded in platform implementation)
- **Windows**: Uses native clipboard change notifications through Windows messaging
- **macOS**: Uses native change notifications via NSPasteboard

### Daemonization

Clipman provides platform-specific daemonization:

- **Linux**: Uses `SetsID` for process group separation
- **Windows**: Creates a hidden window process
- **macOS**: Similar to Linux with macOS-specific parent process detection

## Example Configuration File

```json
{
  "log_level": "info",
  "device_id": "laptop-primary",
  "polling_interval": "500ms",
  "data_dir": "/custom/path/to/data",
  "storage": {
    "db_path": "",
    "max_size": 209715200,
    "keep_items": 50
  },
  "broker": {
    "url": "mqtt://broker.example.com:1883",
    "username": "your_username",
    "password": "your_password"
  },
  "history": {
    "limit": 0,
    "reverse": true,
    "min_size": 0,
    "max_size": 0
  },
  "log": {
    "enable_file_logging": true,
    "max_log_size": 10485760,
    "max_log_files": 10,
    "format": "text"
  }
}
```

## Power User Tips

1. **Environment Variables for Automation**: Set `CLIPMAN_BROKER_URL`, `CLIPMAN_BROKER_USERNAME`, and `CLIPMAN_BROKER_PASSWORD` for quick setup in scripts or containers.

2. **Custom Data Location**: Use `CLIPMAN_DATA_DIR` to store clipboard data on a separate partition or encrypted volume.

3. **Debugging**: Set `CLIPMAN_LOG_LEVEL=debug` to see detailed operation logs.

4. **Multiple Profiles**: Use `--config` flag to point to different config files for different use cases.

5. **Optimize for Memory Usage**: For memory-constrained systems, set a lower max cache size with `--max-size` or in the configuration.

6. **Testing Mode**: Use `clipmand run -d 30s` to run for 30 seconds and view collected clipboard data.

7. **Cache Management**:
   - Use `clipmand flush` to clear old items while keeping the most recent
   - View cache size with `clipmand info`

8. **Cloud Synchronization Security**:
   - Use TLS for broker connections (mqtts://)
   - Set strong, unique passwords
   - Consider using a private MQTT broker
   - Enable ACLs on your MQTT broker to limit access

## Current Limitations

1. The content processor filters and transformers are currently hardcoded and not configurable via the configuration file.

2. The in-memory history size (100 items) is hardcoded and not configurable.

3. Compression is implemented but commented out in the code.

4. The polling interval on Linux (500ms) is hardcoded in the platform implementation.

## Future Enhancement Opportunities

1. Make the content processor filters and transformers configurable.

2. Make the in-memory history size configurable.

3. Enable compression of clipboard content.

4. Make the polling interval configurable.

5. Add support for custom plugins.

6. Add content encryption options for sensitive clipboard data.

7. Implement more sophisticated pruning strategies based on content age and frequency of use. 