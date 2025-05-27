package gui

import (
	"context"
	"fmt"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/gui/ipc"
	"github.com/berrythewa/clipman-daemon/internal/p2p"
	"github.com/berrythewa/clipman-daemon/internal/types"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"go.uber.org/zap"
)

// App represents the main GUI application
type App struct {
	// Core components
	fyneApp    fyne.App
	mainWindow fyne.Window
	ctx        context.Context
	cancel     context.CancelFunc
	config     *config.Config
	logger     *zap.Logger

	// IPC client
	ipcClient *ipc.Client

	// Sync components
	syncManager *p2p.Manager

	// State
	clipboardHistory []*types.ClipboardContent
	pairedDevices   []types.PairedDevice
	syncStatus      *types.SyncStatus
	mainView         *views.MainView
}

// NewApp creates a new GUI application
func NewApp(cfg *config.Config, logger *zap.Logger) (*App, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create Fyne application
	fyneApp := app.New()
	mainWindow := fyneApp.NewWindow("Clipman")

	// Create IPC client
	ipcClient := ipc.NewClient(cfg.IPC.SocketPath)

	// Create sync manager if enabled
	var syncManager *p2p.Manager
	var err error
	if cfg.Sync.Enabled {
		syncManager, err = p2p.New(ctx, cfg, logger)
		if err != nil {
			cancel()
			return nil, fmt.Errorf("failed to create sync manager: %w", err)
		}
	}

	app := &App{
		fyneApp:    fyneApp,
		mainWindow: mainWindow,
		ctx:        ctx,
		cancel:     cancel,
		config:     cfg,
		logger:     logger,
		ipcClient:  ipcClient,
		syncManager: syncManager,
	}

	// Set up the main window
	app.setupMainWindow()

	// Start background tasks
	go app.startBackgroundTasks()

	return app, nil
}

// startBackgroundTasks starts background tasks for updating the UI
func (a *App) startBackgroundTasks() {
	// Update history periodically
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-a.ctx.Done():
				return
			case <-ticker.C:
				history, err := a.ipcClient.GetHistory(100)
				if err != nil {
					a.logger.Error("Failed to get history", zap.Error(err))
					continue
				}
				a.updateHistory(history)
			}
		}
	}()

	// Update sync status periodically
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-a.ctx.Done():
				return
			case <-ticker.C:
				status, err := a.ipcClient.GetSyncStatus()
				if err != nil {
					a.logger.Error("Failed to get sync status", zap.Error(err))
					continue
				}
				a.updateSyncStatus(status)
			}
		}
	}()

	// Update paired devices periodically
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-a.ctx.Done():
				return
			case <-ticker.C:
				devices, err := a.ipcClient.GetPairedDevices()
				if err != nil {
					a.logger.Error("Failed to get paired devices", zap.Error(err))
					continue
				}
				a.updatePairedDevices(devices)
			}
		}
	}()
}

// updateHistory updates the clipboard history
func (a *App) updateHistory(history []*types.ClipboardContent) {
	a.clipboardHistory = history
	if a.mainView != nil {
		a.mainView.UpdateHistory(history)
	}
}

// updateSyncStatus updates the sync status
func (a *App) updateSyncStatus(status *types.SyncStatus) {
	a.syncStatus = status
	if a.mainView != nil {
		a.mainView.UpdateSyncStatus(status)
	}
}

// updatePairedDevices updates the list of paired devices
func (a *App) updatePairedDevices(devices []types.PairedDevice) {
	a.pairedDevices = devices
	if a.mainView != nil {
		a.mainView.UpdatePeers(devices)
	}
}

// Run starts the GUI application
func (a *App) Run() {
	// Start sync manager if available
	if a.syncManager != nil {
		if err := a.syncManager.Start(); err != nil {
			a.logger.Error("Failed to start sync manager", zap.Error(err))
		}
	}

	// Show and run the main window
	a.mainWindow.ShowAndRun()
}

// Shutdown gracefully stops the application
func (a *App) Shutdown() error {
	// Stop sync manager if available
	if a.syncManager != nil {
		if err := a.syncManager.Stop(); err != nil {
			a.logger.Error("Failed to stop sync manager", zap.Error(err))
		}
	}

	// Cancel context
	a.cancel()

	return nil
}

// setupMainWindow configures the main application window
func (a *App) setupMainWindow() {
	// Set window size
	a.mainWindow.Resize(fyne.NewSize(800, 600))

	// Set up the main content
	content := a.createMainContent()
	a.mainWindow.SetContent(content)

	// Set up menu
	a.setupMenu()

	// Set up close handler
	a.mainWindow.SetOnClosed(func() {
		if err := a.Shutdown(); err != nil {
			a.logger.Error("Error during shutdown", zap.Error(err))
		}
	})
}

// createMainContent creates the main window content
func (a *App) createMainContent() fyne.CanvasObject {
	a.mainView = views.NewMainView(a.fyneApp, a.mainWindow)
	return a.mainView.GetContent()
}

// setupMenu sets up the application menu
func (a *App) setupMenu() {
	// TODO: Implement application menu
} 