# Clipman

A cross-platform clipboard manager with history, cloud synchronization, and advanced features.

## Features

- **Clipboard History**: Keep track of your clipboard history and access past items
- **Cross-Platform**: Works on Linux, macOS, and Windows
- **Cloud Sync**: Optionally sync clipboard across devices using MQTT
- **Daemon Mode**: Runs in the background with minimal resource usage
- **Secure**: Keeps your clipboard data private and local by default
- **Extensible**: Plugin architecture for custom transformations

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

# Flush old items from cache
clipman flush
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
  "polling_interval": 1000000000,
  "data_dir": "~/.clipman",
  "storage": {
    "max_size": 104857600,
    "keep_items": 10
  },
  "broker": {
    "url": "",
    "username": "",
    "password": ""
  },
  "log": {
    "enable_file_logging": true,
    "max_log_size": 10485760,
    "max_log_files": 5,
    "format": "text"
  }
}
```

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

MIT License

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.



