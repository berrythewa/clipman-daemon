# Clipman CLI Documentation

Clipman provides a modern command-line interface for managing clipboard operations, history, and configuration. This document outlines the available commands and their usage.

## Command Structure

```bash
clipman [global flags] <command> [command flags] [arguments]
```

### Global Flags

- `--config`: Config file path (default: platform-specific location)
- `--log-level`: Set logging level (debug, info, warn, error)

## Commands

### 1. Daemon Management (`clipman daemon`)

Manage the Clipman daemon process that handles clipboard monitoring and syncing.

```bash
# Start the daemon
clipman daemon start [--detach]

# Stop the daemon
clipman daemon stop [--force]

# Check daemon status
clipman daemon status

# Restart the daemon
clipman daemon restart [--force]
```

#### Flags:
- `--detach`: Run in background mode (for start)
- `--force`: Force operation (for stop/restart)

### 2. Clipboard Operations (`clipman clip`)

Perform direct clipboard operations.

```bash
# Get current clipboard content
clipman clip get [--raw]

# Set clipboard content
clipman clip set [--type=<type>] <content>
clipman clip set < file.txt  # Set from stdin

# Watch clipboard changes (monitors in real-time)
clipman clip watch [--timeout=<duration>]

# Flush clipboard history
clipman clip flush [--keep=<n>]
```

#### Flags:
- `--raw`: Output raw content without metadata (for get)
- `--type, -t`: Content type (text, image, file, url, etc.) (for set)
- `--timeout`: Watch timeout duration (for watch)
- `--keep, -k`: Number of recent items to keep (for flush)

### 3. History Management (`clipman history`)

Manage clipboard history entries with advanced filtering and search capabilities.

```bash
# List history entries
clipman history list [flags]

# Show specific entry by hash
clipman history show <hash> [--raw]

# Delete entries
clipman history delete <hash> [--all] [--older=<duration>]

# Show history statistics
clipman history stats [--json]
```

#### List Flags:
- `--limit, -n`: Maximum number of entries to show (default: 50)
- `--search`: Search for specific text within clipboard content
- `--type, -t`: Filter by content type (text, image, file, url, html, rtf)
- `--since`: Show entries since duration (e.g., 24h, 7d)
- `--before`: Show entries before duration
- `--verbose, -v`: Show detailed metadata for each entry
- `--min-size`: Minimum content size in bytes
- `--max-size`: Maximum content size in bytes
- `--reverse, -r`: Reverse order (newest first)

#### Delete Flags:
- `--all`: Delete all history
- `--older`: Delete entries older than duration (e.g., 30d)
- `--type`: Delete entries of specific type

#### Statistics Output
The `stats` command provides detailed information about your clipboard usage:
- Total number of items stored
- Storage size and compression statistics
- Content type distribution
- Most frequent content items
- Usage patterns over time

### 4. Configuration Management (`clipman config`)

Manage Clipman configuration settings stored in YAML format.

```bash
# Show current configuration
clipman config show [--format=<yaml|json>]

# Edit configuration in your preferred editor
clipman config edit

# Reset configuration to defaults
clipman config reset [--force]

# Validate configuration syntax
clipman config validate
```

#### Flags:
- `--format, -f`: Output format for show command (yaml or json, default: yaml)
- `--force`: Force overwrite existing config (for reset)

#### Configuration File Locations:
- **Linux**: `~/.config/clipman/config.yaml`
- **macOS**: `~/Library/Application Support/com.berrythewa.clipman/config.yaml`
- **Windows**: `%APPDATA%\Clipman\config.yaml`

### 5. Device Pairing (`clipman pair`) 🔄 **In Development**

Manage secure device pairing for P2P synchronization.

```bash
# Enter pairing mode (makes device discoverable)
clipman pair [--timeout=<seconds>] [--auto-accept]

# Request pairing with another device
clipman pair --request <address>

# List all paired devices
clipman pair --list

# Remove a paired device
clipman pair --remove <peer-id>
```

#### Flags:
- `--timeout`: Automatically exit pairing mode after specified time
- `--auto-accept`: Automatically accept all pairing requests (use with caution)
- `--request`: Request pairing with device at specified address
- `--list`: List all securely paired devices
- `--remove`: Remove a paired device by Peer ID

*Note: Device pairing functionality is currently in development. Basic pairing protocols are implemented but sync functionality is still being completed.*

### 6. Service Management (`clipman service`) ✅ **Linux Only**

Manage system service integration (currently Linux/systemd only).

```bash
# Install systemd service
clipman service install [--user]

# Uninstall systemd service
clipman service uninstall [--user]

# Enable service to start at boot
clipman service enable [--user]

# Disable service
clipman service disable [--user]

# Check service status
clipman service status [--user]
```

#### Flags:
- `--user`: Manage user service instead of system service

### 7. Version Information (`clipman version`)

Display version and build information.

```bash
# Show version information
clipman version [--json]
```

## Content Types

Clipman automatically detects and supports various content types:

- **`text`**: Plain text content (UTF-8)
- **`image`**: Binary image data (PNG, JPEG, GIF, etc.)
- **`file`**: File paths or file lists (local file references)
- **`url`**: URLs and web links
- **`html`**: HTML formatted content
- **`rtf`**: Rich text format content

Content type detection is automatic, but you can specify the type manually when setting clipboard content.

## Examples

### Basic Usage
```bash
# Start daemon and begin monitoring
clipman daemon start --detach

# Copy text to clipboard
clipman clip set "Hello, World!"

# Get current clipboard
clipman clip get

# List recent clipboard history
clipman history list --limit 10

# Search for specific content
clipman history list --search "password" --type text
```

### Advanced History Management
```bash
# Show detailed information for recent items
clipman history list --verbose --limit 5

# Find large images from the last week
clipman history list --type image --since 7d --min-size 100000

# Show statistics about clipboard usage
clipman history stats --json

# Clean up old entries (keep last 50 items)
clipman history delete --older 30d

# Show specific item by hash
clipman history show abc123def456
```

### Configuration Management
```bash
# View current configuration
clipman config show

# Edit configuration file
clipman config edit

# Reset to defaults
clipman config reset --force

# Validate configuration
clipman config validate
```

### Service Management (Linux)
```bash
# Install as user service
clipman service install --user

# Enable automatic startup
clipman service enable --user

# Check service status
clipman service status --user
```

## Environment Variables

The following environment variables can be used to override default behavior:

- **`EDITOR`**: Preferred text editor for config editing (default: vim)
- **`CLIPMAN_CONFIG`**: Override default config file location
- **`CLIPMAN_DATA_DIR`**: Override default data directory location
- **`CLIPMAN_LOG_LEVEL`**: Set logging level (debug, info, warn, error)
- **`XDG_CONFIG_HOME`**: Alternative config directory base (Linux)
- **`XDG_DATA_HOME`**: Alternative data directory base (Linux)

## Exit Codes

- **0**: Success
- **1**: General error
- **2**: Configuration error
- **3**: Daemon error (not running, failed to start, etc.)
- **4**: Permission error
- **5**: Network/sync error

## Usage Notes

### Prerequisites
1. **Daemon Required**: Most commands require the daemon to be running
2. **Platform Dependencies**: 
   - Linux: `xclip`, `xsel`, or `wl-clipboard` required
   - macOS: Native support (no dependencies)
   - Windows: Native support (no dependencies)

### Performance Considerations
- **Large Content**: Items over 10MB are automatically compressed
- **History Size**: Default limit is 1000 items, configurable in settings
- **Polling**: Linux uses adaptive polling (100ms-500ms interval) when events aren't available

### Security Notes
- **Local Storage**: All clipboard data is stored locally by default
- **Encryption**: Sync traffic between devices is encrypted (when implemented)
- **Permissions**: Daemon requires clipboard access permissions on some platforms

### Common Workflows

#### Daily Use
```bash
# Start monitoring (run once)
clipman daemon start --detach

# Normal clipboard usage (automatic)
# Use Ctrl+C/Ctrl+V as usual

# Retrieve old content
clipman history list --search "important text"
clipman clip set "$(clipman history show abc123def)"
```

#### Development
```bash
# Monitor clipboard changes in real-time
clipman clip watch

# Copy command output
echo "some output" | clipman clip set

# Archive current clipboard
clipman clip get > important_clipboard.txt
```

#### Maintenance
```bash
# Check storage usage
clipman history stats

# Clean up old entries
clipman history delete --older 30d

# Reset configuration
clipman config reset && clipman daemon restart
```

For more detailed configuration options, see [CONFIGURATION.md](CONFIGURATION.md).

For device pairing and synchronization, see [PAIRING.md](PAIRING.md). 