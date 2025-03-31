package cmd

import (
	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/storage"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// flushCmd represents the flush command
var flushCmd = &cobra.Command{
	Use:   "flush",
	Short: "Flush the clipboard cache",
	Long: `Force a flush of the clipboard cache to free up space.
This will keep the most recent items based on the keep-items setting.

The command will display the clipboard history before and after the flush
operation, unless the --quiet flag is used.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		zapLogger.Info("Forcing clipboard cache flush")
		
		// Get all system paths
		paths := cfg.GetSystemPaths()
		// Initialize storage
		storageConfig := storage.StorageConfig{
			DBPath:   paths.DBFile,
			DeviceID: cfg.DeviceID,
			Logger:   zapLogger,
			MaxSize:  cfg.Storage.MaxSize,
		}
		
		store, err := storage.NewBoltStorage(storageConfig)
		if err != nil {
			zapLogger.Error("Failed to initialize storage", zap.Error(err))
			return err
		}
		defer store.Close()
		
		// First log the history before flush, unless quiet mode is on
		if !quiet {
			zapLogger.Info("History before flush:")
			if err := store.LogCompleteHistory(config.HistoryOptions{}); err != nil {
				zapLogger.Error("Failed to dump history before flush", zap.Error(err))
			}
		}
		
		// Get cache size before flush
		cacheSizeBefore := store.GetCacheSize()
		
		// Now flush the cache
		if err := store.FlushCache(); err != nil {
			zapLogger.Error("Failed to flush cache", zap.Error(err))
			return err
		}
		
		// Get cache size after flush
		cacheSizeAfter := store.GetCacheSize()
		
		// Log flushing results
		zapLogger.Info("Cache flushed successfully", 
			zap.Int64("freed_bytes", cacheSizeBefore - cacheSizeAfter),
			zap.Int64("cache_size_before", cacheSizeBefore),
			zap.Int64("cache_size_after", cacheSizeAfter))
		
		// Log the history after flush, unless quiet mode is on
		if !quiet {
			zapLogger.Info("History after flush:")
			if err := store.LogCompleteHistory(config.HistoryOptions{}); err != nil {
				zapLogger.Error("Failed to dump history after flush", zap.Error(err))
			}
		}
		
		return nil
	},
}

func init() {
	// Set up flags for this command
	flushCmd.Flags().BoolVar(&quiet, "quiet", false, "Don't display history before and after flush")
}