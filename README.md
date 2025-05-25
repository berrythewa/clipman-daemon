# Clipman

A secure, cross-platform clipboard manager and sync suite for power users, developers, and privacy-conscious individuals. Clipman goes beyond basic clipboard history by offering encrypted synchronization, file sharing, and a modern CLI interface.

## Features

### Core Functionality (Implemented)

* **Modern CLI Interface**: Intuitive command structure for daemon management, clipboard operations, history, and configuration.
* **Cross-Platform Support**: Native implementations for Linux (X11), macOS (NSPasteboard), and Windows (Clipboard API).
* **Clipboard History**: Stores and organizes clipboard entries with content type detection and metadata.
* **Efficient Storage**: Uses BoltDB for persistent storage with automatic pruning and size management.
* **Content Types**: Support for text, images, files, URLs, HTML, and RTF content.
* **Daemon Mode**: Runs quietly in the background with platform-specific implementations.
* **Advanced History Management**: Filter by type, time, size, with support for JSON output.

### Platform-Specific Features

#### Linux Implementation
* X11 clipboard integration using `github.com/atotto/clipboard`
* Polling-based monitoring with adaptive intervals (500ms - 2s)
* Systemd service integration for daemon management

#### macOS Implementation
* NSPasteboard integration for native clipboard access
* Change count monitoring for efficient updates
* Launchd service integration for daemon management

#### Windows Implementation
* Windows Clipboard API integration
* Event-based monitoring using `WM_CLIPBOARDUPDATE`
* Windows Service support for daemon management

### Storage & Caching

* **Persistent Storage**: BoltDB-based storage with:
  - Configurable size limits (default 100MB)
  - Automatic pruning of old content
  - Content deduplication
  - Occurrence tracking
* **Memory Management**: 
  - Atomic size tracking
  - Configurable retention policies
  - Efficient content retrieval

## Installation & Usage

### Command Line Interface

```bash
clipman [global flags] <command> [command flags] [arguments]
```

#### Global Flags
- `--config`: Config file path
- `--verbose`: Enable verbose output
- `--quiet`: Minimize output
- `--json`: Output in JSON format

#### Core Commands

1. **Daemon Management**
```bash
clipman daemon start [--background]
clipman daemon stop [--force]
clipman daemon status
clipman daemon restart [--force]
```

2. **Clipboard Operations**
```bash
clipman clip get [--raw] [--json]
clipman clip set [--type=<type>] <content>
clipman clip watch [--timeout=<duration>]
clipman clip flush [--keep=<n>]
```

3. **History Management**
```bash
clipman history list [--limit=N] [--type=TYPE] [--since=DURATION]
clipman history show <hash> [--raw]
clipman history delete [--all] [--older=DURATION]
clipman history stats [--json]
```

4. **Configuration Management**
```bash
clipman config show [--json]
clipman config edit
clipman config reset [--force]
```

### Configuration

Default configuration paths:
- Linux: `~/.config/clipman/config.yaml`
- macOS: `~/Library/Application Support/Clipman/config.yaml`
- Windows: `%APPDATA%\Clipman\config.yaml`

## Development Status

### Implemented
- ✅ Modern CLI framework with consistent command structure
- ✅ Cross-platform clipboard monitoring
- ✅ Efficient storage engine with BoltDB
- ✅ Content type detection and handling
- ✅ Platform-specific daemon management
- ✅ History management with filtering
- ✅ Configuration management

### In Progress
- 🔄 Platform-specific clipboard implementations
  - Linux: Basic implementation complete
  - macOS: Structure ready, NSPasteboard integration pending
  - Windows: Structure ready, Win32 API integration pending
- 🔄 Daemon process management
  - Linux: Basic implementation with systemd
  - macOS: Launchd integration pending
  - Windows: Service integration pending

### Planned
- 📋 Secure P2P sync with libp2p
- 📋 Device pairing and room management
- 📋 End-to-end encryption
- 📋 Secret vault integration
- 📋 Mobile platform support (Android/iOS)

## Contributing

Contributions are welcome! Please check the issues page for current tasks or create a new issue to discuss proposed changes.

## License

Licensed under MIT. See LICENSE file for details.

---

> ⚡ Built with Go, BoltDB, and platform-native clipboard APIs.
> 
> 📦 Zero external dependencies for core clipboard functionality.
> 
> 🔒 Privacy-first design with local-first architecture.
