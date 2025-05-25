# Clipman

A secure, cross-platform clipboard manager and sync suite with integrated secret vault and P2P file sharing. Built for power users, developers, and privacy-conscious individuals who need a unified solution for managing sensitive data across devices.

## Core Features

### Clipboard Management (Implemented)
* **Modern CLI Interface**: Intuitive command structure for all operations
* **Cross-Platform Support**: 
  - Desktop: Linux (X11), macOS (NSPasteboard), Windows (Win32 API)
  - Mobile: Android and iOS (in development)
* **Smart History**: 
  - Content type detection (text, images, files, URLs, HTML, RTF)
  - Metadata tracking and deduplication
  - Advanced filtering by type, time, and size
* **Efficient Storage**: 
  - BoltDB-based with automatic pruning
  - Configurable retention policies
  - Atomic operations and size management

### Secure Sync & Sharing (In Development)
* **P2P Architecture**:
  - libp2p-based direct device communication
  - No central servers or cloud dependencies
  - LAN-first with optional WAN fallback
* **End-to-End Encryption**:
  - Zero-knowledge design
  - Perfect forward secrecy
  - Quantum-resistant algorithms
* **Secure Rooms**:
  - Create private sharing spaces
  - Role-based access control
  - Optional ephemeral mode
  - Built-in secure chat
* **File Sharing**:
  - Direct device-to-device transfer
  - Automatic compression
  - Resume-capable transfers
  - Integrity verification

### Secret Vault (Planned)
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
* **Security Features**:
  - Biometric unlock
  - Master password
  - Emergency access
  - Audit logging

## Platform-Specific Implementation

### Desktop Support
#### Linux (Implemented)
* X11 clipboard integration via `github.com/atotto/clipboard`
* Adaptive polling (500ms - 2s intervals)
* Systemd service integration

#### macOS (In Progress)
* Native NSPasteboard integration
* Change count monitoring
* Launchd service management

#### Windows (In Progress)
* Windows Clipboard API
* Event-based monitoring
* Windows Service support

### Mobile Support (Planned)
#### Android
* Native clipboard service
* Background sync service
* Accessibility service for auto-fill

#### iOS
* Universal clipboard integration
* Extension for auto-fill
* Background app refresh

## Architecture

### Storage Layer
* **Local Storage**:
  - BoltDB for clipboard and vault
  - Encrypted at rest
  - Configurable size limits
  - Automatic pruning

### Network Layer
* **P2P Communication**:
  - libp2p for device discovery
  - NAT traversal
  - Relay support for difficult networks
  - Bandwidth management

### Security Layer
* **Encryption**:
  - ChaCha20-Poly1305 for symmetric
  - X25519 for key exchange
  - Ed25519 for signatures
  - Argon2id for key derivation

## Usage

### Command Line Interface
```bash
clipman [global flags] <command> [flags] [arguments]
```

For detailed CLI documentation, see [CLI.md](docs/CLI.md)

### Core Commands
```bash
# Daemon Management
clipman daemon {start|stop|status|restart}

# Clipboard Operations
clipman clip {get|set|watch|flush}

# History Management
clipman history {list|show|delete|stats}

# Configuration
clipman config {show|edit|reset}

# Vault Operations (Coming Soon)
clipman vault {unlock|lock|list|add|remove}

# Room Management (Coming Soon)
clipman room {create|join|leave|list|chat}
```

## Development Status

### Implemented ✅
- Modern CLI framework
- Cross-platform clipboard monitoring
- Efficient storage engine
- Content type detection
- History management
- Configuration system

### In Progress 🔄
- Platform-specific clipboard implementations
- Daemon process management
- P2P networking foundation
- End-to-end encryption
- Mobile platform support

### Planned 📋
- Secret vault integration
- Secure rooms and chat
- File sharing system
- Browser extensions
- Mobile apps

## Contributing

We welcome contributions! Check our issues page for current tasks or create a new issue to discuss proposed changes.

## License

Licensed under MIT. See LICENSE file for details.

---

> ⚡ Built with Go, libp2p, and BoltDB
> 
> 🔒 Zero-knowledge, end-to-end encrypted
> 
> 📱 Cross-platform, from desktop to mobile
> 
> 🌐 P2P-first, cloud-optional architecture
