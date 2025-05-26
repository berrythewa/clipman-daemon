# Clipman CLI Documentation

Clipman provides a modern command-line interface for managing clipboard operations, history, and configuration. This document outlines the available commands and their usage.

## Command Structure

```bash
clipman [global flags] <command> [command flags] [arguments]
```

### Global Flags

- `--config`: Config file path (default: `$HOME/.config/clipman/config.yaml`)
- `--verbose`: Enable verbose output
- `--quiet`: Minimize output
- `--json`: Output in JSON format

## Commands

### 1. Daemon Management (`clipman daemon`)

Manage the Clipman daemon process that handles clipboard monitoring and syncing.

```bash
# Start the daemon
clipman daemon start [--background]

# Stop the daemon
clipman daemon stop [--force]

# Check daemon status
clipman daemon status

# Restart the daemon
clipman daemon restart [--force]
```

#### Flags:
- `--background, -b`: Run in background (for start)
- `--force, -f`: Force operation (for stop/restart)

### 2. Clipboard Operations (`clipman clip`)

Perform direct clipboard operations.

```bash
# Get current clipboard content
clipman clip get [--raw] [--json]

# Set clipboard content
clipman clip set [--type=<type>] <content>
clipman clip set < file.txt  # Set from stdin

# Watch clipboard changes
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

Manage clipboard history entries.

```bash
# List history entries
clipman history list [flags]

# Show specific entry
clipman history show <hash> [--raw]

# Delete entries
clipman history delete [hash...] [flags]

# Show history statistics
clipman history stats [--json]
```

#### List Flags:
- `--limit, -n`: Maximum number of entries to show
- `--since`: Show entries since duration (e.g., 24h)
- `--before`: Show entries before duration
- `--reverse, -r`: Reverse order (newest first)
- `--type, -t`: Filter by content type
- `--min-size`: Minimum content size in bytes
- `--max-size`: Maximum content size in bytes

#### Delete Flags:
- `--all`: Delete all history
- `--older`: Delete entries older than duration
- `--type`: Delete entries of specific type

### 4. Configuration Management (`clipman config`)

Manage Clipman configuration. The configuration is stored in YAML format with comprehensive settings for all aspects of the application. For detailed configuration options and examples, see [CONFIGURATION.md](CONFIGURATION.md).

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
- `--force, -f`: Force overwrite existing config (for reset)

#### Configuration Sections:
```yaml
# Main configuration sections:
device:        # Device identification settings
paths:         # System paths configuration
clipboard:     # Clipboard monitoring settings
history:       # History management options
storage:       # Storage and database settings
logging:       # Logging configuration
security:      # Security and encryption settings
system:        # System integration options
development:   # Development and debug settings
```

#### Example Configuration Paths:
- Linux: `~/.config/clipman/config.yaml`
- macOS: `~/Library/Application Support/Clipman/config.yaml`
- Windows: `%APPDATA%\Clipman\config.yaml`

## Content Types

Clipman supports various content types:
- `text`: Plain text content
- `image`: Binary image data
- `file`: File paths or file lists
- `url`: URLs and web links
- `html`: HTML content
- `rtf`: Rich text format content

## Examples

```bash
# Start daemon in background
clipman daemon start -b

# Copy text to clipboard
clipman clip set "Hello, World!"

# Watch clipboard changes for 1 hour
clipman clip watch --timeout=1h

# List last 5 clipboard entries
clipman history list -n 5 --reverse

# Show statistics in JSON format
clipman history stats --json

# Show current config in JSON format
clipman config show --format=json

# Edit configuration
clipman config edit

# Reset config to defaults (with confirmation)
clipman config reset
```

## Environment Variables

- `EDITOR`: Preferred text editor for config editing (default: vim)
- `HOME`: User's home directory for default paths
- `XDG_CONFIG_HOME`: Alternative config directory base
- `XDG_DATA_HOME`: Alternative data directory base
- `CLIPMAN_CONFIG`: Override default config file location
- `CLIPMAN_DATA_DIR`: Override default data directory location

## Exit Codes

- 0: Success
- 1: General error
- 2: Configuration error
- 3: Daemon error
- 4: Permission error

## Notes

1. The daemon must be running for most commands to work
2. Some operations may require elevated privileges when using system-wide features
3. All timestamps are in local time zone
4. JSON output is available for most commands that return data
5. The clipboard history is stored in a local database with configurable size limits
6. Configuration changes require a daemon restart to take effect
7. The configuration file is extensively documented with comments explaining each option
8. Invalid configuration files will be detected during validation but won't prevent saving
9. For a complete list of configuration options and their descriptions, see [CONFIGURATION.md](CONFIGURATION.md) 