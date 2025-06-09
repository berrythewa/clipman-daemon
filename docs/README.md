# Clipman Documentation

This directory contains detailed documentation for Clipman, a cross-platform clipboard manager with P2P synchronization and advanced features.

## Documentation Index

### User Documentation

- [**CLI Reference**](CLI.md) - Complete command-line interface documentation
- [**Configuration Guide**](CONFIGURATION.md) - Comprehensive overview of all configuration options
- [**Device Pairing**](PAIRING.md) - Guide to securely connecting devices for synchronization
- [**Platform Support**](PLATFORM_IMPLEMENTATIONS.md) - Implementation details for Linux, macOS, and Windows

### Development Documentation

- [**Development Roadmap**](TODO.md) - Current development priorities and planned features

## Quick Start Guide

### Basic Setup
1. **Install Clipman**: Follow the installation instructions in the [main README](../README.md)
2. **Start the daemon**: `clipman daemon start`
3. **Use the clipboard**: Your clipboard is now being monitored and stored

### Basic Commands
```bash
# Get current clipboard
clipman clip get

# List clipboard history
clipman history list

# Search history
clipman history list --search "search term"

# Configure settings
clipman config show
clipman config edit
```

### Device Synchronization
1. **Enable sync** in your configuration file
2. **Start pairing** on first device: `clipman pair`
3. **Connect from second device**: `clipman pair --request <address>`
4. **Verify codes** match on both devices
5. **Your devices are now paired** and will sync automatically

For detailed pairing instructions, see [PAIRING.md](PAIRING.md).

## Core Features

### ✅ **Currently Available**
- **Clipboard Management**: Monitor, store, and retrieve clipboard history
- **Cross-Platform Support**: Complete Linux implementation with X11/Wayland support
- **Advanced History**: Search, filter, and manage clipboard content
- **Daemon Architecture**: Robust background process with IPC communication
- **Configuration System**: Comprehensive YAML-based configuration
- **Device Pairing**: Secure P2P device pairing and trust establishment

### 🔄 **In Development**
- **P2P Synchronization**: Real-time clipboard sync between paired devices
- **macOS Support**: Native clipboard integration for macOS
- **Windows Support**: Native clipboard integration for Windows
- **Enhanced Security**: End-to-end encryption for synchronized content

### 📋 **Planned**
- **Secret Vault**: Password manager and secure notes
- **Mobile Support**: Android and iOS applications
- **Browser Extensions**: Web integration and auto-fill
- **File Sharing**: Secure file transfer between devices

## Configuration At a Glance

Clipman's configuration is managed through:

1. **Command-line flags** (highest priority)
2. **Environment variables**
3. **Configuration file** (YAML format)
4. **Built-in defaults** (lowest priority)

### Configuration File Locations
- **Linux**: `~/.config/clipman/config.yaml`
- **macOS**: `~/Library/Application Support/com.berrythewa.clipman/config.yaml`
- **Windows**: `%APPDATA%\Clipman\config.yaml`

### Key Configuration Sections
```yaml
device:        # Device identification
clipboard:     # Clipboard monitoring settings
history:       # History management
storage:       # Database and storage settings
sync:          # P2P synchronization settings
logging:       # Logging configuration
security:      # Security and encryption
```

For complete configuration details, see [CONFIGURATION.md](CONFIGURATION.md).

## Platform Support

### Linux ✅ **Complete**
- X11 and Wayland clipboard support
- Event-based monitoring with adaptive polling fallback
- Systemd service integration
- Complete feature set available

### macOS 🔄 **In Progress**
- Platform detection implemented
- Native NSPasteboard integration in development
- Launchd service support planned

### Windows 🔄 **In Progress**
- Platform detection implemented
- Windows Clipboard API integration in development
- Windows Service support planned

For detailed platform information, see [PLATFORM_IMPLEMENTATIONS.md](PLATFORM_IMPLEMENTATIONS.md).

## Command Categories

### Daemon Management
```bash
clipman daemon start    # Start background daemon
clipman daemon stop     # Stop daemon
clipman daemon status   # Check daemon status
```

### Clipboard Operations
```bash
clipman clip get        # Get current clipboard
clipman clip set        # Set clipboard content
clipman clip watch      # Monitor clipboard changes
```

### History Management
```bash
clipman history list    # List clipboard history
clipman history show    # Show specific item
clipman history stats   # Display statistics
```

### Device Pairing
```bash
clipman pair           # Enter pairing mode
clipman pair --request # Connect to device
clipman pair --list    # List paired devices
```

### Configuration
```bash
clipman config show    # Show current configuration
clipman config edit    # Edit configuration file
```

For complete command documentation, see [CLI.md](CLI.md).

## Architecture Overview

Clipman is built with a modular architecture:

- **CLI Interface**: User-facing command interface built with Cobra
- **Daemon Process**: Background service handling clipboard monitoring
- **Storage Layer**: BoltDB-based persistent storage with deduplication
- **Platform Layer**: Platform-specific clipboard and system integration
- **P2P Layer**: libp2p-based networking for device synchronization
- **Configuration**: Hierarchical configuration system with YAML files

## Security Model

- **Local-First**: Your data stays on your devices by default
- **Secure Pairing**: Device-to-device trust establishment with verification
- **P2P Communication**: Direct device communication without cloud dependencies
- **Encryption**: End-to-end encryption for synchronized content (in development)
- **Zero-Knowledge**: No central servers can access your clipboard data

## Getting Help

If you encounter issues or have questions:

1. **Check this documentation** - Start with the relevant guide above
2. **Review configuration** - Use `clipman config show` to verify settings
3. **Check daemon status** - Use `clipman daemon status` to ensure the daemon is running
4. **Review logs** - Check log files in your system's log directory
5. **Open an issue** - Report bugs or request features on GitHub

## Contributing to Documentation

Documentation improvements are welcome! When contributing:

1. **Keep it current** - Ensure information matches the actual implementation
2. **Be comprehensive** - Include examples and edge cases
3. **Use clear language** - Write for users of all technical levels
4. **Test examples** - Verify all command examples work correctly
5. **Update cross-references** - Keep links between documents current

---

*Documentation last updated for Clipman development version. Features marked as "In Development" or "Planned" may not be available in released versions.* 