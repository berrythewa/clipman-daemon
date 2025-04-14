# 📦 Clipman Sync Configuration

This document describes the configuration options available for customizing the behavior of the Clipman sync engine, powered by [libp2p](https://libp2p.io/). You can control how clipboard and file syncing works across local networks and the internet.

---

## 🛠️ Core Sync Settings

| Setting                          | Type       | Description |
|----------------------------------|------------|-------------|
| `enable_sync`                    | `bool`     | Enable or disable syncing entirely |
| `sync_over_internet`            | `bool`     | Allow peer discovery and sync over WAN |
| `use_relay_nodes`               | `bool`     | Enable fallback to libp2p relay nodes if direct connection fails |
| `listen_port`                   | `int`      | Override the default TCP port (defaults to dynamic/random) |
| `peer_identity`                 | `string`   | Exportable identity for restoring or sharing (advanced users) |
| `discovery_method`              | `string`   | Choose between `mdns`, `dht`, `manual`, or `paired` discovery |

---

## 🔗 Peer Discovery & Persistence

| Setting                          | Type       | Description |
|----------------------------------|------------|-------------|
| `persist_discovered_peers`      | `bool`     | Save discovered peers to disk for future sessions |
| `discovered_peers_path`         | `string`   | Path to store discovered peers information |
| `auto_reconnect_to_peers`       | `bool`     | Automatically connect to previously discovered peers |
| `max_stored_peers`              | `int`      | Maximum number of peers to store (oldest removed first) |
| `dht_bootstrap_peers`           | `array`    | List of bootstrap peers for DHT discovery |
| `dht_persistent_storage`        | `bool`     | Whether to store DHT data on disk |
| `dht_storage_path`              | `string`   | Path to store DHT routing table data |
| `dht_server_mode`               | `bool`     | Run DHT in server mode (more resource usage but helps network) |

---

## 🤝 Secure Pairing Options

| Setting                          | Type       | Description |
|----------------------------------|------------|-------------|
| `pairing_enabled`               | `bool`     | Enable or disable secure device pairing functionality |
| `pairing_timeout`               | `int`      | Seconds before pairing mode times out (0 for no timeout) |
| `require_verification`          | `bool`     | Whether verification codes must be confirmed for pairing |
| `device_name`                   | `string`   | Human-readable name for this device shown during pairing |
| `device_type`                   | `string`   | Type of device (desktop, mobile, etc.) |

---

## 🖥️ Clipboard Sync Options

| Setting                          | Type       | Description |
|----------------------------------|------------|-------------|
| `clipboard_types`               | `array`    | Types to sync: `["text", "image", "files"]` |
| `auto_copy_from_peers`          | `bool`     | Automatically copy received clipboard content to local clipboard |
| `max_clipboard_size_kb`         | `int`      | Maximum clipboard item size in KB to sync (default: `512`) |
| `clipboard_history_size`        | `int`      | Number of clipboard entries to keep in sync history |
| `clipboard_blacklist_apps`      | `array`    | Prevent syncing clipboard contents from specific applications |

---

## 📂 File Transfer Options

| Setting                          | Type       | Description |
|----------------------------------|------------|-------------|
| `enable_file_sharing`           | `bool`     | Enable or disable peer-to-peer file transfers |
| `require_file_confirmation`     | `bool`     | Ask for confirmation before accepting incoming files |
| `default_download_folder`       | `string`   | Path to folder where received files should be saved |
| `auto_accept_from_peers`        | `array`    | List of trusted PeerIDs that can send files automatically |
| `max_file_size_mb`              | `int`      | Maximum file size (in MB) allowed for transfer |

---

## 🔐 Privacy & Security

| Setting                          | Type       | Description |
|----------------------------------|------------|-------------|
| `allow_only_known_peers`        | `bool`     | Disable open discovery and only sync with approved/trusted peers |
| `trusted_peers`                 | `array`    | PeerIDs trusted for auto clipboard/file actions |
| `require_approval_pin`          | `bool`     | Ask for a PIN or approval before syncing content |
| `log_peer_activity`             | `bool`     | Log peer connections and sync events (for debugging and audit) |

---

## 🧪 Developer & Debug Options

| Setting                          | Type       | Description |
|----------------------------------|------------|-------------|
| `debug_logging`                 | `bool`     | Enable verbose debug logging |
| `show_peer_debug_info`          | `bool`     | Show info about discovered peers (ID, addresses) |
| `disable_multiplexing`          | `bool`     | Disable stream multiplexing (use raw streams) |
| `force_direct_connection_only`  | `bool`     | Disable relays and test only direct connections |

---

## 📝 Example Configuration (TOML)

```toml
# Core Sync Settings
enable_sync = true
sync_over_internet = true
use_relay_nodes = true
listen_port = 4001
discovery_method = "paired"

# Peer Discovery & Persistence
persist_discovered_peers = true
discovered_peers_path = "~/.clipman/peers.json"
auto_reconnect_to_peers = true
max_stored_peers = 100
dht_bootstrap_peers = []
dht_persistent_storage = false
dht_storage_path = "~/.clipman/dht"
dht_server_mode = false

# Device Information
device_name = "My Linux Desktop"
device_type = "desktop"

# Secure Pairing Options
pairing_enabled = true
pairing_timeout = 300
require_verification = true

# Clipboard Sync Options
clipboard_types = ["text", "image"]
auto_copy_from_peers = true
max_clipboard_size_kb = 1024
clipboard_history_size = 50
clipboard_blacklist_apps = ["keepassxc", "bitwarden"]

# File Transfer Options
enable_file_sharing = true
require_file_confirmation = true
default_download_folder = "~/Downloads/Clipman"
max_file_size_mb = 100
auto_accept_from_peers = ["QmPeerID1", "QmPeerID2"]

# Privacy & Security
allow_only_known_peers = true
trusted_peers = ["QmTrustedPeer"]
require_approval_pin = false
log_peer_activity = true

# Developer & Debug Options
debug_logging = false
show_peer_debug_info = false

```

## 🔍 Discovery Methods

### `paired` (Recommended)
Uses the secure device pairing protocol to connect only with devices that have been explicitly paired through a manual pairing process. This is the most secure option and recommended for most users.

### `mdns`
Uses multicast DNS (mDNS) to discover peers on the local network automatically. No configuration required, but only works on the local network.

### `dht`
Uses a Distributed Hash Table (DHT) to discover peers across the internet. Requires bootstrap peers and may use more bandwidth.

### `manual`
Only connects to peers that have been manually added by address. Most restrictive option but gives complete control.
