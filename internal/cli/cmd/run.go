package cmd

import (
	"time"
	"context"

	"github.com/berrythewa/clipman-daemon/internal/clipboard"
	// "github.com/berrythewa/clipman-daemon/internal/config"
	// TODO: check why not needed
	"github.com/berrythewa/clipman-daemon/internal/storage"
	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/berrythewa/clipman-daemon/internal/sync"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var (
	duration time.Duration
	maxSize  int64
	noSync   bool
)

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the clipboard monitor daemon",
	Long: `Run the clipboard monitor daemon which watches for clipboard changes
and stores them in the history database.

You can specify a duration for testing purposes, otherwise it will run
until interrupted.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		zapLogger.Info("Starting Clipman daemon",
			zap.String("device_id", cfg.DeviceID),
			zap.Bool("sync_enabled", cfg.Sync.Enabled),
			zap.String("discovery_method", cfg.Sync.DiscoveryMethod),
			zap.Bool("internet_sync", cfg.Sync.SyncOverInternet),
			zap.Int("listen_port", cfg.Sync.ListenPort),
			zap.Int64("port", cfg.Server.Port),
			zap.String("host", cfg.Server.Host),
			zap.String("path", cfg.Server.Path),
			zap.String("username", cfg.Server.Username))
		
		// If maxSize is specified, override config
		if maxSize > 0 {
			cfg.Storage.MaxSize = maxSize
		}

		// Get all system paths
		paths := cfg.GetPaths()
		
		// Initialize content publisher
		var contentPublisher clipboard.ContentPublisher
		
		if noSync || !cfg.Sync.Enabled {
			zapLogger.Info("Using no-op publisher (sync functionality disabled)")
			contentPublisher = clipboard.NewNoOpPublisher(zapLogger)
		} else {
			// Create sync manager with the global config
			syncManager, err := sync.New(context.Background(), cfg, zapLogger)
			if err != nil {
				zapLogger.Error("Failed to initialize sync manager, falling back to no-op publisher", zap.Error(err))
				contentPublisher = clipboard.NewNoOpPublisher(zapLogger)
			} else {
				// Start the sync manager
				if err := syncManager.Start(); err != nil {
					zapLogger.Error("Failed to start sync manager, falling back to no-op publisher", zap.Error(err))
					contentPublisher = clipboard.NewNoOpPublisher(zapLogger)
				} else {
					// Join the default group
					defaultGroup := "clipman-default"
					if err := syncManager.JoinGroup(defaultGroup); err != nil {
						zapLogger.Error("Failed to join default group, falling back to no-op publisher", zap.Error(err))
						contentPublisher = clipboard.NewNoOpPublisher(zapLogger)
					} else {
						zapLogger.Info("Using sync publisher",
							zap.String("group", defaultGroup))
						contentPublisher = clipboard.NewSyncPublisher(syncManager, defaultGroup, zapLogger)
					}
				}
			}
		}
		
		// Initialize storage
		storageConfig := storage.StorageConfig{
			DBPath:     paths.DBFile,
			MaxSize:    cfg.Storage.MaxSize,
			DeviceID:   cfg.DeviceID,
			Logger:     zapLogger,
		}
		
		zapLogger.Info("Storage configuration", 
			zap.String("db_path", paths.DBFile),
			zap.Int64("max_size_bytes", storageConfig.MaxSize),
			zap.String("device_id", cfg.DeviceID))
			
		store, err := storage.NewBoltStorage(storageConfig)
		if err != nil {
			zapLogger.Error("Failed to initialize storage", zap.Error(err))
			return err
		}
		defer store.Close()
		
		// Start the monitor
		monitor := clipboard.NewMonitor(cfg, contentPublisher, zapLogger, store)
		if err := monitor.Start(); err != nil {
			zapLogger.Error("Failed to start monitor", zap.Error(err))
			return err
		}
		
		zapLogger.Info("Monitor started")
		
		if duration > 0 {
			// Run for specified duration
			zapLogger.Info("Running for test duration", zap.Duration("duration", duration))
			time.Sleep(duration)
			
			// Log the complete history after the test duration
			zapLogger.Info("Test complete, logging clipboard history")
			if err := store.LogCompleteHistory(cfg.History); err != nil {
				zapLogger.Error("Failed to log history", zap.Error(err))
			}
			
			zapLogger.Info("Stopping monitor")
			monitor.Stop()
			
			// Flush logger to ensure all logs are written
			zapLogger.Sync()
			
			// Show recent items
			recentHistory := monitor.GetHistory(10)
			for _, item := range recentHistory {
				dataPreview := "binary data"
				if item.Content.Type == types.TypeText || item.Content.Type == types.TypeURL {
					previewLength := min(len(item.Content.Data), 50)
					dataPreview = string(item.Content.Data[:previewLength])
				}
				
				zapLogger.Info("Recent clipboard item",
					zap.String("type", string(item.Content.Type)),
					zap.Time("time", item.Time),
					zap.Int("data_length", len(item.Content.Data)),
					zap.String("preview", dataPreview))
			}
		} else {
			// Run indefinitely - block until interrupted
			zapLogger.Info("Running until interrupted, press Ctrl+C to stop")
			select {}
		}
		
		return nil
	},
}

func init() {
	// Set up flags for this command
	runCmd.Flags().DurationVarP(&duration, "duration", "d", 0, "Run for a specific duration (for testing)")
	runCmd.Flags().Int64Var(&maxSize, "max-size", 0, "Override max cache size in bytes (default 100MB)")
	runCmd.Flags().BoolVar(&noSync, "no-sync", false, "Disable sync connection even if configured")
}

// Helper function to get minimum of two values
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
} 