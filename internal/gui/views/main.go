package views

import (
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

	// State
	history []*types.ClipboardContent
	app     fyne.App
	window  fyne.Window
}

// NewMainView creates a new main view
func NewMainView(app fyne.App, window fyne.Window) *MainView {
	view := &MainView{
		app:    app,
		window: window,
		history: make([]*types.ClipboardContent, 0),
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

	// Create status bar
	v.statusBar = widget.NewLabel("Ready")

	// Create main layout
	content := container.NewBorder(
		v.toolbar,    // top
		v.statusBar,  // bottom
		nil,          // left
		nil,          // right
		v.historyList, // center
	)

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
			)
		},
		func(id widget.ListItemID, item fyne.CanvasObject) {
			content := v.history[id]
			box := item.(*fyne.Container)
			label := box.Objects[1].(*widget.Label)

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

// UpdateHistory updates the clipboard history
func (v *MainView) UpdateHistory(history []*types.ClipboardContent) {
	v.history = history
	v.historyList.Refresh()
}

// UpdateStatus updates the status bar
func (v *MainView) UpdateStatus(status string) {
	v.statusBar.SetText(status)
}

// copyToClipboard copies content to the system clipboard
func (v *MainView) copyToClipboard(content *types.ClipboardContent) {
	// TODO: Implement clipboard copy
}

// showSettings shows the settings view
func (v *MainView) showSettings() {
	// TODO: Implement settings view
}

// showPairing shows the pairing view
func (v *MainView) showPairing() {
	// TODO: Implement pairing view
}

// clearHistory clears the clipboard history
func (v *MainView) clearHistory() {
	// TODO: Implement history clearing
} 