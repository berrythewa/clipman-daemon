# Clipman

A secure, cross-platform clipboard manager and sync suite with integrated secret vault and P2P file sharing. Built for power users, developers, and privacy-conscious individuals who need a unified solution for managing sensitive data across devices.

## Quick Start

### Prerequisites
- Go 1.23 or later
- Platform-specific dependencies:
  - **Linux**: X11 development libraries (`libx11-dev`, `libxfixes-dev`) or Wayland (`wl-clipboard`)
  - **macOS**: Xcode command-line tools
  - **Windows**: No additional dependencies
- Git

### Installation

#### Local Installation (No Root Required)
```bash
# Clone the repository
git clone https://github.com/berrythewa/clipman-daemon.git
cd clipman-daemon

# Build and install locally
make install-local
```

#### System-wide Installation
```bash
# Build and install system-wide
sudo make install
```

#### Using Go Install
```bash
go install github.com/berrythewa/clipman-daemon/cmd/clipman@latest
go install github.com/berrythewa/clipman-daemon/cmd/clipmand@latest
```

### Running the Daemon

#### Manual Start
```bash
# Start in foreground
clipman daemon start

# Start in background (detached)
clipman daemon start --detach

# Check status
clipman daemon status

# Stop the daemon
clipman daemon stop
```

#### Systemd Service (Linux)
```bash
# Install systemd service
sudo make install
sudo systemctl enable clipman@$USER
sudo systemctl start clipman@$USER

# Check service status
systemctl status clipman@$USER
```

### Basic Usage

```bash
# Get current clipboard content
clipman clip get

# Set clipboard content
clipman clip set "Hello, World!"

# List clipboard history
clipman history list

# Show detailed history with metadata
clipman history list --verbose --limit 20

# Search history by content
clipman history list --search "search term"

# Configure the daemon
clipman config show
clipman config edit
```

### Uninstallation

#### System-wide Uninstallation
```bash
# Stop services and remove installation
sudo make uninstall
```

#### Local Uninstallation
```bash
# Remove local installation
rm -f ~/.local/bin/clipman ~/.local/bin/clipmand

# Remove data and configuration (optional)
rm -rf ~/.clipman ~/.config/clipman ~/.local/share/clipman
```

## Core Features

### Clipboard Management ✅ **Implemented**
* **Modern CLI Interface**: Intuitive command structure with comprehensive help
* **Cross-Platform Support**: 
  - **Linux**: Complete X11 and Wayland support with event-based monitoring
  - **macOS**: Platform detection implemented, native NSPasteboard integration in progress
  - **Windows**: Platform detection implemented, Win32 API integration in progress
* **Smart History Management**: 
  - Content type detection (text, images, files, URLs, HTML, RTF)
  - Metadata tracking with occurrence frequency
  - Advanced deduplication and hash-based storage
  - Configurable retention policies and automatic cleanup
* **Efficient Storage**: 
  - BoltDB-based persistence with atomic operations
  - Intelligent compression for large content
  - Size-based cache management with automatic pruning
  - Multi-level indexing for fast retrieval

### Advanced Clipboard Features ✅ **Implemented**
* **Content Type Detection**: Automatic recognition of text, images, files, URLs, HTML, and RTF
* **Occurrence Tracking**: Frequency-based content ranking and smart suggestions
* **Search Capabilities**: Full-text search across clipboard history
* **Format Preservation**: Maintains original content formatting and metadata
* **Size Optimization**: Compression and efficient storage for large content

### Secure Sync & Sharing 🔄 **In Progress**
* **P2P Architecture**:
  - libp2p-based direct device communication *(Core implemented)*
  - No central servers or cloud dependencies *(Working)*
  - LAN-first with optional internet relay *(In development)*
* **Device Discovery**:
  - mDNS local network discovery *(Implemented)*
  - DHT-based internet discovery *(In progress)*
  - Manual peer addition *(Working)*
  - Secure device pairing *(Core implemented)*
* **End-to-End Encryption**:
  - Zero-knowledge design *(Architecture complete)*
  - Perfect forward secrecy *(In development)*
  - ChaCha20-Poly1305 encryption *(Planned)*
* **Sync Features**:
  - Real-time clipboard synchronization *(In progress)*
  - Conflict resolution *(Planned)*
  - Selective content filtering *(Planned)*

### System Integration ✅ **Implemented**
* **Daemon Architecture**: Robust daemon process with proper lifecycle management
* **IPC Communication**: Efficient inter-process communication between CLI and daemon
* **Platform Services**: 
  - Linux systemd integration *(Complete)*
  - macOS launchd support *(Planned)*
  - Windows service support *(Planned)*
* **Configuration Management**: Comprehensive YAML-based configuration system
* **Logging System**: Structured logging with configurable levels and file rotation

### Secret Vault 📋 **Planned**
* **Password Manager**:
  - Secure credential storage
  - Auto-fill support
  - Password generation
  - Breach monitoring
* **Secure Notes**:
  - Encrypted note storage
  - Tags and categories
  - Version history
* **Two-Factor Authentication**:
  - TOTP generator
  - Hardware key support
  - Backup codes management

## Development

### Building from Source
```bash
# Build both CLI and daemon
make build-all

# Build with debug information
make build-debug

# Build for specific platform
make build-cli
make build-daemon

# Clean build artifacts
make clean
```

### Development Tools
```bash
# Run all tests
make test

# Run tests with race detection
make test-race

# Run linters
make lint

# Update dependencies
make deps

# Check build environment
make check-env
```

### Release Building
```bash
# Build release packages for all platforms
make release
```

## Platform Support

### Desktop Support
#### Linux ✅ **Complete**
* Native X11 clipboard integration via `xclip`/`xsel`
* Wayland support via `wl-clipboard`
* Event-based monitoring with XFixes extension
* Adaptive polling fallback for compatibility
* Systemd service integration
* Proper resource cleanup and signal handling

#### macOS 🔄 **In Progress**
* Platform detection and configuration *(Complete)*
* Native NSPasteboard integration *(In development)*
* Change count monitoring *(Planned)*
* Launchd service management *(Planned)*

#### Windows 🔄 **In Progress**
* Platform detection and configuration *(Complete)*
* Windows Clipboard API integration *(In development)*
* Event-based monitoring *(Planned)*
* Windows Service support *(Planned)*

### Mobile Support 📋 **Planned**
#### Android
* Native clipboard service integration
* Background sync service
* Accessibility service for auto-fill

#### iOS
* Universal clipboard integration
* App extension for auto-fill
* Background app refresh support

## Architecture

### Storage Layer ✅ **Implemented**
* **Local Storage**:
  - BoltDB for clipboard history and metadata
  - Configurable encryption at rest
  - Automatic size management and pruning
  - Hash-based deduplication
  - Occurrence frequency tracking

### Network Layer 🔄 **In Progress**
* **P2P Communication**:
  - libp2p for device discovery and communication
  - NAT traversal with relay support
  - Bandwidth management and optimization
  - Protocol versioning and compatibility

### Security Layer 🔄 **In Progress**
* **Encryption**:
  - ChaCha20-Poly1305 for symmetric encryption *(Planned)*
  - X25519 for key exchange *(Planned)*
  - Ed25519 for digital signatures *(Planned)*
  - Argon2id for key derivation *(Planned)*

### Configuration System ✅ **Implemented**
* **Hierarchical Configuration**:
  - Command-line flags (highest priority)
  - Environment variables
  - YAML configuration file
  - Built-in defaults (lowest priority)
* **Platform-Specific Paths**:
  - Linux: XDG Base Directory specification
  - macOS: Apple Application Support directories
  - Windows: AppData directories

## Usage

### Command Line Interface

```bash
# Global structure
clipman [global flags] <command> [subcommand] [flags] [arguments]
```

### Available Commands

#### Daemon Management
```bash
clipman daemon start [--detach]    # Start daemon
clipman daemon stop               # Stop daemon  
clipman daemon status             # Check status
clipman daemon restart            # Restart daemon
```

#### Clipboard Operations
```bash
clipman clip get                  # Get current clipboard
clipman clip set "text"           # Set clipboard content
clipman clip watch               # Watch clipboard changes
clipman clip flush               # Clear clipboard
```

#### History Management
```bash
clipman history list [--limit N] [--search term] [--type text|image|file]
clipman history show <hash>      # Show specific item
clipman history delete <hash>    # Delete specific item
clipman history stats            # Show statistics
```

#### Configuration
```bash
clipman config show             # Show current config
clipman config edit             # Edit config file
clipman config reset            # Reset to defaults
```

#### P2P and Sync *(In Development)*
```bash
clipman pair start              # Start pairing mode
clipman pair list               # List paired devices
clipman sync enable             # Enable synchronization
clipman sync status             # Show sync status
```

## Configuration

### Configuration File Locations
- **Linux**: `~/.config/clipman/config.yaml`
- **macOS**: `~/Library/Application Support/com.berrythewa.clipman/config.yaml`
- **Windows**: `%APPDATA%\Clipman\config.yaml`

### Key Configuration Options
```yaml
# Device identification
device:
  name: "My Device"
  id: "auto-generated"

# Clipboard monitoring
clipboard:
  monitor: true
  content_types: [text, image, file, url, html]
  max_content_size: 10485760  # 10MB

# History settings
history:
  max_items: 1000
  max_size: 104857600  # 100MB
  deduplication:
    enabled: true

# Sync settings (when implemented)
sync:
  enabled: false
  discovery_method: "paired"
  require_approval_pin: false
```

## Development Status

### Recently Completed ✅
- Complete Linux clipboard implementation with X11/Wayland support
- Robust daemon architecture with proper process management
- Comprehensive CLI with all core clipboard operations
- BoltDB storage engine with advanced features
- Configuration system with platform-specific paths
- Build system with cross-compilation support
- Systemd service integration

### Current Work 🔄
- P2P networking foundation (libp2p integration ~80% complete)
- Device pairing and discovery protocols
- macOS and Windows platform implementations
- End-to-end encryption framework
- Sync protocol implementation

### Next Priorities 📋
- Complete P2P synchronization functionality
- macOS and Windows native clipboard integration
- Mobile platform architecture planning
- Secret vault foundation
- Browser extension framework

## Contributing

We welcome contributions! The project follows standard Go development practices:

1. **Code Style**: Follow `gofmt` and `golangci-lint` standards
2. **Testing**: Include tests for new functionality
3. **Documentation**: Update relevant documentation
4. **Platform Support**: Test on target platforms when possible

### Development Setup
```bash
git clone https://github.com/berrythewa/clipman-daemon.git
cd clipman-daemon
make deps
make build-all
make test
```

## License

Licensed under MIT. See [LICENSE](LICENSE) file for details.

---

> ⚡ **Built with Go, libp2p, and BoltDB** - Modern, efficient, and reliable
> 
> 🔒 **Security First** - Zero-knowledge architecture with end-to-end encryption
> 
> 📱 **Cross-Platform** - Native implementations from desktop to mobile
> 
> 🌐 **P2P-First** - Direct device communication, cloud-optional by design
> 
> 🚀 **Production Ready** - Robust daemon architecture with comprehensive testing

### Status Legend
- ✅ **Implemented**: Feature is complete and fully functional
- 🔄 **In Progress**: Feature is partially implemented or under active development  
- 📋 **Planned**: Feature is designed and scheduled for future implementation
