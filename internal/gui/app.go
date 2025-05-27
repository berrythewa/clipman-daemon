package gui

import (
	"context"
	"fmt"

	"github.com/berrythewa/clipman-daemon/internal/config"
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

	// Sync components
	syncManager *p2p.Manager

	// State
	clipboardHistory []*types.ClipboardContent
	pairedDevices   []types.PairedDevice
}

// NewApp creates a new GUI application
func NewApp(cfg *config.Config, logger *zap.Logger) (*App, error) {
	ctx, cancel := context.WithCancel(context.Background())

	// Create Fyne application
	fyneApp := app.New()
	mainWindow := fyneApp.NewWindow("Clipman")

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
		syncManager: syncManager,
	}

	// Set up the main window
	app.setupMainWindow()

	return app, nil
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
	// TODO: Implement main content layout
	return nil
}

// setupMenu sets up the application menu
func (a *App) setupMenu() {
	// TODO: Implement application menu
} 