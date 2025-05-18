# Clipman

A secure, cross-platform clipboard manager and sync suite for power users, developers, and privacy-conscious individuals. Clipman goes beyond basic clipboard history by offering encrypted synchronization, file sharing, a secret vault, and more.

## Features

### Core Functionality

* **Clipboard History**: Automatically stores and organizes clipboard entries with support for text, images, and more.
* **Cross-Platform**: Available for Linux, macOS, Windows (desktop) — with mobile support in development (Android/iOS).
* **Secure P2P Sync**: End-to-end encrypted peer-to-peer synchronization powered by libp2p and optional DHT/mDNS discovery.
* **Device Pairing**: Seamless and secure pairing process using verifiable device addresses and shared secrets.
* **Daemon Mode**: Runs quietly in the background, optimized for system resources.
* **History Filtering**: View history by type, time, device, or size with advanced filtering options.

### Advanced Security Features

* **End-to-End Encryption**: All communications and data storage are zero-knowledge encrypted using secure cryptographic standards.
* **Secret Vault** (Work in Progress): A built-in password and secrets manager with per-item encryption and optional biometrics.
* **Selective Sync**: Choose what gets synced (e.g., only text, only a specific room, only from/to trusted devices).
* **Zero Metadata Mode**: Optionally strips all metadata from shared content.

### Device Collaboration & Sharing

* **Rooms**: Create temporary or persistent sharing spaces between devices or users.
* **Secure File Sharing**: Drop-in support for file clipboard sync and drag-and-drop between devices.
* **Chat in Rooms** (Planned): Lightweight secure chat between paired devices within rooms.

### Local-first Philosophy

* **Offline Mode**: Full access to clipboard history and vault without internet access.
* **LAN-only Sync**: Sync across devices on the same local network without touching the internet.
* **Private Storage**: Uses BoltDB for local storage with pruning, compression, and customizable retention.

## Optional Growth Features (Planned)

* **Clipboard Actions**: Automatic actions triggered by copied content (e.g., open link, beautify JSON, detect emails).
* **Secure Share Links**: Generate time-limited encrypted links for sharing history items or files.
* **Search & Tagging**: Powerful local search across history with custom tags.
* **API / CLI Tooling**: Programmatic access to clipboard, vault, and file sync via REST or CLI.
* **Biometric Unlock**: Unlock secret vaults using Face ID, Touch ID, or YubiKey.
* **Self-hosted Version**: Docker-based deployable instance for total control in team or enterprise environments.
* **Browser Extensions**: Integration for autofill, password insertion, and clipboard sync from the browser.
* **Encrypted Audit Logs**: User-controlled logs of access and operations (stored locally, optional).

---

## Installation & Usage

Install, configure, and run Clipman on your platform with ease. Refer to the installation and command sections below for full details.

> For detailed CLI usage, daemon options, auto-start configuration, and device pairing, see the full documentation inside the repo.

---

Clipman is designed to give you complete control over your data without sacrificing usability. Whether you're syncing secrets, sharing files, or just want to search your clipboard from last week, Clipman has your back.

---

> ⚡ Powered by Go, libp2p, BoltDB, and a user-focused privacy-first design.

Licensed under MIT. Contributions welcome!
