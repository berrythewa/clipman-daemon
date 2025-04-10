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
| `discovery_method`              | `string`   | Choose between `mdns`, `dht`, or `manual` discovery |

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
enable_sync = true
sync_over_internet = true
use_relay_nodes = true
listen_port = 4001
discovery_method = "mdns"

clipboard_types = ["text", "image"]
auto_copy_from_peers = true
max_clipboard_size_kb = 1024
clipboard_history_size = 50
clipboard_blacklist_apps = ["keepassxc", "bitwarden"]

enable_file_sharing = true
require_file_confirmation = true
default_download_folder = "~/Downloads/Clipman"
max_file_size_mb = 100
auto_accept_from_peers = ["QmPeerID1", "QmPeerID2"]

allow_only_known_peers = false
trusted_peers = ["QmTrustedPeer"]
require_approval_pin = false
log_peer_activity = true

debug_logging = false
show_peer_debug_info = false
