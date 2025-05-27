# Clipman GUI

This package provides the graphical user interface for Clipman, built using Fyne toolkit.

## Structure

```
gui/
├── app.go          # Main application setup and lifecycle
├── window.go       # Main window management
├── views/          # Different UI views/screens
│   ├── main.go     # Main view with clipboard history
│   ├── settings.go # Settings view
│   └── pairing.go  # Device pairing view
├── widgets/        # Custom widgets
│   ├── history.go  # Clipboard history widget
│   └── peer.go     # Peer list widget
└── theme/          # Custom theme and styling
    └── theme.go    # Theme definitions
```

## Features

- Clipboard history display
- Device pairing interface
- Settings management
- Real-time sync status
- Peer management
- File transfer interface

## Dependencies

- [Fyne](https://fyne.io/) - Cross-platform GUI toolkit
- [Fyne-io/fyne](https://github.com/fyne-io/fyne) - Core Fyne package
- [Fyne-io/fyne/v2](https://github.com/fyne-io/fyne/tree/v2) - Fyne v2 API

## Usage

The GUI can be launched using the `clipman gui` command. It provides a user-friendly interface for managing clipboard synchronization and device pairing. 