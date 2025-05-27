package views

import (
	"fmt"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/types"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/widget"
)

// MainView represents the main application view
type MainView struct {
	// UI components
	historyList *widget.List
	statusBar   *widget.Label
	toolbar     *widget.Toolbar
	peerList    *widget.List
	syncStatus  *widget.Label

	// State
	history []*types.ClipboardContent
	peers   []types.PairedDevice
	app     fyne.App
	window  fyne.Window
}

// NewMainView creates a new main view
func NewMainView(app fyne.App, window fyne.Window) *MainView {
	view := &MainView{
		app:    app,
		window: window,
		history: make([]*types.ClipboardContent, 0),
		peers:   make([]types.PairedDevice, 0),
	}

	view.createUI()
	return view
}

// createUI creates the main view UI
func (v *MainView) createUI() {
	// Create toolbar
	v.createToolbar()

	// Create history list
	v.createHistoryList()

	// Create peer list
	v.createPeerList()

	// Create status bar
	v.statusBar = widget.NewLabel("Ready")
	v.syncStatus = widget.NewLabel("Sync: Disconnected")

	// Create main layout
	content := container.NewHSplit(
		container.NewBorder(
			v.toolbar,    // top
			v.statusBar,  // bottom
			nil,          // left
			nil,          // right
			v.historyList, // center
		),
		container.NewBorder(
			widget.NewLabel("Paired Devices"), // top
			v.syncStatus,                      // bottom
			nil,                               // left
			nil,                               // right
			v.peerList,                        // center
		),
	)

	// Set split ratio
	content.SetOffset(0.7) // 70% for history, 30% for peers

	v.window.SetContent(content)
}

// createToolbar creates the main toolbar
func (v *MainView) createToolbar() {
	v.toolbar = widget.NewToolbar(
		widget.NewToolbarAction(theme.SettingsIcon(), v.showSettings),
		widget.NewToolbarAction(theme.PairingIcon(), v.showPairing),
		widget.NewToolbarSeparator(),
		widget.NewToolbarAction(theme.ClearIcon(), v.clearHistory),
	)
}

// createHistoryList creates the clipboard history list
func (v *MainView) createHistoryList() {
	v.historyList = widget.NewList(
		func() int { return len(v.history) },
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewIcon(theme.DocumentIcon()),
				widget.NewLabel("Template"),
				widget.NewLabel(""), // Timestamp
			)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			content := v.history[id]
			box := item.(*fyne.Container)
			label := box.Objects[1].(*widget.Label)
			timestamp := box.Objects[2].(*widget.Label)

			// Set content preview
			switch content.Type {
			case types.ContentTypeText:
				label.SetText(content.Preview)
			case types.ContentTypeImage:
				label.SetText("Image")
			case types.ContentTypeFiles:
				label.SetText("Files")
			default:
				label.SetText("Unknown content type")
			}

			// Set timestamp
			timestamp.SetText(content.Timestamp.Format("15:04:05"))
		},
	)

	// Set up selection handler
	v.historyList.OnSelected = func(id widget.ListItemID) {
		if id < 0 || id >= len(v.history) {
			return
		}
		v.copyToClipboard(v.history[id])
	}
}

// createPeerList creates the peer list
func (v *MainView) createPeerList() {
	v.peerList = widget.NewList(
		func() int { return len(v.peers) },
		func() fyne.CanvasObject {
			return container.NewHBox(
				widget.NewIcon(theme.ComputerIcon()),
				widget.NewLabel("Template"),
				widget.NewLabel(""), // Status
			)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			peer := v.peers[id]
			box := item.(*fyne.Container)
			label := box.Objects[1].(*widget.Label)
			status := box.Objects[2].(*widget.Label)

			label.SetText(peer.Name)
			if peer.Connected {
				status.SetText("Connected")
				status.TextStyle = fyne.TextStyle{Bold: true}
			} else {
				status.SetText("Disconnected")
				status.TextStyle = fyne.TextStyle{}
			}
		},
	)
}

// UpdateHistory updates the clipboard history
func (v *MainView) UpdateHistory(history []*types.ClipboardContent) {
	v.history = history
	v.historyList.Refresh()
}

// UpdatePeers updates the peer list
func (v *MainView) UpdatePeers(peers []types.PairedDevice) {
	v.peers = peers
	v.peerList.Refresh()
}

// UpdateSyncStatus updates the sync status
func (v *MainView) UpdateSyncStatus(status *types.SyncStatus) {
	if status == nil {
		v.syncStatus.SetText("Sync: Disconnected")
		return
	}

	var statusText string
	switch status.State {
	case types.SyncStateConnected:
		statusText = fmt.Sprintf("Sync: Connected (%d peers)", status.ConnectedPeers)
	case types.SyncStateConnecting:
		statusText = "Sync: Connecting..."
	case types.SyncStateDisconnected:
		statusText = "Sync: Disconnected"
	default:
		statusText = "Sync: Unknown"
	}

	v.syncStatus.SetText(statusText)
}

// UpdateStatus updates the status bar
func (v *MainView) UpdateStatus(status string) {
	v.statusBar.SetText(status)
}

// copyToClipboard copies content to the system clipboard
func (v *MainView) copyToClipboard(content *types.ClipboardContent) {
	// TODO: Implement clipboard copy through IPC
	v.UpdateStatus(fmt.Sprintf("Copied: %s", content.Preview))
}

// showSettings shows the settings view
func (v *MainView) showSettings() {
	// TODO: Implement settings view
	v.UpdateStatus("Settings: Not implemented yet")
}

// showPairing shows the pairing view
func (v *MainView) showPairing() {
	// TODO: Implement pairing view
	v.UpdateStatus("Pairing: Not implemented yet")
}

// clearHistory clears the clipboard history
func (v *MainView) clearHistory() {
	// TODO: Implement history clearing through IPC
	v.UpdateStatus("History cleared")
} 