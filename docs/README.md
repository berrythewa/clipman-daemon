# Clipman Documentation

This directory contains detailed documentation for Clipman, a cross-platform clipboard manager with history, cloud synchronization, and advanced features.

## Documentation Index

### User Documentation

- [**Configuration Guide**](CONFIGURATION.md) - Comprehensive overview of all configurable options in Clipman
- [**Platform Implementations**](PLATFORM_IMPLEMENTATIONS.md) - Details on how Clipman implements functionality on Linux, macOS, and Windows
- [**Caching System**](CACHING.md) - Deep dive into Clipman's multi-level caching architecture

### For Developers

If you're interested in contributing to Clipman or understanding its internal architecture, these documents are particularly useful:

- [**Platform Implementations**](PLATFORM_IMPLEMENTATIONS.md) - Details of platform-specific code
- [**Caching System**](CACHING.md) - Explains the storage and caching architecture

## Main Features

- **Clipboard History**: Keep track of your clipboard history and access past items
- **Cross-Platform**: Works on Linux, macOS, and Windows with native implementations
- **Cloud Sync**: Optionally sync clipboard across devices using MQTT
- **Daemon Mode**: Runs in the background with minimal resource usage
- **Secure**: Keeps your clipboard data private and local by default
- **Platform-Specific**: Optimized for each supported operating system
- **Efficient Storage**: Uses BoltDB for efficient, persistent clipboard storage
- **Extensible**: Modular architecture for easy enhancement

## Quick Links

- [Project README](../README.md)
- [Configuration Guide](CONFIGURATION.md)
- [Caching System](CACHING.md)
- [Platform Implementations](PLATFORM_IMPLEMENTATIONS.md)

## Configuration At a Glance

Clipman's configuration can be customized through:

1. **Command-line flags**
2. **Environment variables**
3. **Configuration file** (JSON format)

The configuration file is automatically created at platform-specific locations:

- **Linux**: `~/.config/clipman/config.json`
- **macOS**: `~/Library/Application Support/com.berrythewa.clipman/config.json`
- **Windows**: `%APPDATA%\Clipman\config.json`

For a complete list of configuration options, see the [Configuration Guide](CONFIGURATION.md).

## Getting Help

If you encounter issues or have questions:

1. Check the documentation in this directory
2. Refer to the main [README](../README.md)
3. Open an issue on the GitHub repository 