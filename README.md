# Clipman

A cross-platform clipboard manager with history, peer-to-peer synchronization, and advanced features.

## Features

- **Clipboard History**: Keep track of your clipboard history and access past items
- **Cross-Platform**: Works on Linux, macOS, and Windows with native implementations
- **Secure P2P Sync**: Sync clipboard across devices using secure libp2p connections
- **Device Pairing**: Easily pair devices with a secure verification process
- **Daemon Mode**: Runs in the background with minimal resource usage
- **Secure**: End-to-end encrypted communication with paired devices
- **Platform-Specific**: Optimized for each supported operating system
- **Efficient Storage**: Uses BoltDB for efficient, persistent clipboard storage
- **Extensible**: Modular architecture for easy enhancement

## Architecture

Clipman is built with a clean, modular architecture:

- **Platform Package**: Contains platform-specific implementations for:
  - Clipboard monitoring on Linux, Windows, and macOS
  - Daemonization techniques appropriate for each OS
- **Storage Layer**: Uses BoltDB for efficient, persistent storage with:
  - Configurable cache size
  - Automatic pruning of old entries
  - Optional data compression
- **CLI Interface**: Clean command structure using Cobra
  - Global daemon command
  - History viewing and filtering
  - Cache management
  - System service installation
- **Sync Package**: Peer-to-peer synchronization using libp2p
  - Device discovery via mDNS, DHT, or manual configuration
  - Secure device pairing with verification
  - End-to-end encrypted communication
  - Device-centric content synchronization

## Installation

### Quick Install

#### Linux/macOS

```bash
# Using the installation script
curl -fsSL https://raw.githubusercontent.com/berrythewa/clipman-daemon/main/scripts/install.sh | sudo bash

# Or clone the repository and install
git clone https://github.com/berrythewa/clipman-daemon.git
cd clipman-daemon
make install
```

#### Windows

```powershell
# Download and run the installation script (Run as Administrator)
Invoke-WebRequest -Uri https://raw.githubusercontent.com/berrythewa/clipman-daemon/main/scripts/install.ps1 -OutFile install.ps1
powershell -ExecutionPolicy Bypass -File install.ps1
```

### Build from Source

```bash
git clone https://github.com/berrythewa/clipman-daemon.git
cd clipman-daemon
make build

# The binary will be in bin/clipman
```

## Usage

Clipman is designed to be simple to use. By default, running `clipman` with no arguments starts the daemon in the foreground.

### Basic Commands

```bash
# Start the daemon in the foreground
clipman

# Start the daemon in the background
clipman --detach

# View clipboard history
clipman history

# Display the last 5 items
clipman history --limit 5

# Show clipboard history in reverse order (newest first)
clipman history --reverse

# Filter clipboard history by type
clipman history --type text

# Show clipboard history from a specific time range
clipman history --since "2023-06-01" --before "2023-06-30"

# Flush old items from cache
clipman flush
```

### Device Pairing and Sync

```bash
# Enable pairing mode on the first device
clipman pair

# Use the displayed address to pair from second device
clipman pair --connect "address_from_first_device"

# List paired devices
clipman devices list

# View clipboard history from a specific device
clipman content --from-device "laptop-work"

# Remove a paired device
clipman devices remove "device_id"
```

### Command-Line Options

#### Global Options

```
--config string     Config file path (default is system-specific)
--device-id string  Override device ID
--log-level string  Log level (debug, info, warn, error)
--no-file-log       Disable logging to file
```

#### Daemon Mode Options

```
--detach           Run in the background
--no-sync          Disable synchronization even if configured
--max-size int64   Override max cache size in bytes
```

#### History Options

```
--limit int        Maximum number of entries to display
--since string     Display entries after this time
--before string    Display entries before this time
--type string      Filter by content type (text, image, etc.)
--min-size int     Filter by minimum size in bytes
--reverse          Show newest entries first
--json             Output in JSON format
```

### Setting Up Auto-Start

Clipman can be installed as a system service to start automatically:

```bash
# Install as a user service (starts for current user only)
clipman service install

# Install as a system service (starts for all users)
clipman service install --system

# Check service status
clipman service status

# Uninstall the service
clipman service uninstall
```

## Configuration

The default configuration file is stored in:

- Linux: `~/.config/clipman/config.json`
- macOS: `~/Library/Application Support/clipman/config.json`
- Windows: `%APPDATA%\clipman\config.json`

The clipboard database is stored in:

- Linux: `~/.clipman/clipboard.db`
- macOS: `~/Library/Application Support/clipman/clipboard.db`
- Windows: `%USERPROFILE%\.clipman\clipboard.db`

### Configuration Options

```json
{
  "log_level": "info",
  "device_id": "auto-generated-uuid",
  "device_name": "my-laptop",
  "device_type": "laptop",
  "polling_interval": 1000000000,
  "data_dir": "~/.clipman",
  "storage": {
    "max_size": 104857600,
    "keep_items": 10
  },
  "sync": {
    "enable_sync": true,
    "sync_over_internet": false,
    "discovery_method": "mdns",
    "listen_port": 0,
    "allow_only_known_peers": true,
    "clipboard_types": ["text", "image"]
  },
  "log": {
    "enable_file_logging": true,
    "max_log_size": 10485760,
    "max_log_files": 5,
    "format": "text"
  },
  "history": {
    "limit": 0,
    "reverse": false
  }
}
```

## Device Synchronization

Clipman uses a peer-to-peer approach for syncing clipboard content across devices:

1. **Device Pairing**: Securely pair devices using a verification code
2. **Discovery**: Devices find each other using various methods:
   - mDNS for local network discovery
   - DHT for Internet-based discovery (when enabled)
   - Manual connection using device addresses
3. **Secure Communication**: All communications between devices are end-to-end encrypted
4. **Device-Centric Model**: View and access clipboard content from specific paired devices
5. **Selective Sync**: Control which content types are synchronized

### Pairing Process

1. Enable pairing mode on the first device: `clipman pair`
2. Note the displayed address and verification code
3. On the second device, run: `clipman pair --connect "address_from_first_device"`
4. Verify that both devices show the same verification code
5. Confirm the pairing on both devices
6. Devices are now paired and can sync clipboard content

### Discovery Methods

Clipman supports multiple methods for discovering paired devices:

- **mDNS**: Local network discovery (default, works well on home/office networks)
- **DHT**: Distributed Hash Table for Internet-based discovery
- **Manual**: Direct connection using known addresses
- **Paired**: Connect only to previously paired devices

## Platform-Specific Implementation Details

### Linux
- Uses polling for clipboard monitoring (X11 limitations)
- Daemonization with SetsID for process group separation
- Systemd service integration

### macOS
- Uses clipboard change count APIs for efficient monitoring
- Launchd integration for service management
- Handles both pasteboard and services API

### Windows
- Uses Windows Clipboard Listener for efficient event-based monitoring
- Windows service integration
- Hidden window technique for background operation

## Uninstalling

To uninstall Clipman:

```bash
# Linux/macOS
curl -fsSL https://raw.githubusercontent.com/berrythewa/clipman-daemon/main/scripts/uninstall.sh | sudo bash

# Or if installed from source
cd clipman-daemon
make uninstall
```

```powershell
# Windows (Run as Administrator)
Invoke-WebRequest -Uri https://raw.githubusercontent.com/berrythewa/clipman-daemon/main/scripts/uninstall.ps1 -OutFile uninstall.ps1
powershell -ExecutionPolicy Bypass -File uninstall.ps1
```

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

### Development Workflow

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests: `make test`
5. Submit a pull request



