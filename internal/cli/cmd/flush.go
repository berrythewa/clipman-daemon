package cmd

import (
	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/storage"
	"github.com/spf13/cobra"
)

var (
	quiet bool
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
		logger.Info("Forcing clipboard cache flush")
		
		// Get all system paths
		paths := cfg.GetPaths()
		
		// Initialize storage
		storageConfig := storage.StorageConfig{
			DBPath:   paths.DBFile,
			DeviceID: cfg.DeviceID,
			Logger:   logger,
			MaxSize:  cfg.Storage.MaxSize,
		}
		
		store, err := storage.NewBoltStorage(storageConfig)
		if err != nil {
			logger.Error("Failed to initialize storage", "error", err)
			return err
		}
		defer store.Close()
		
		// First log the history before flush, unless quiet mode is on
		if !quiet {
			logger.Info("History before flush:")
			if err := store.LogCompleteHistory(config.HistoryOptions{}); err != nil {
				logger.Error("Failed to dump history before flush", "error", err)
			}
		}
		
		// Get cache size before flush
		cacheSizeBefore := store.GetCacheSize()
		
		// Now flush the cache
		if err := store.FlushCache(); err != nil {
			logger.Error("Failed to flush cache", "error", err)
			return err
		}
		
		// Get cache size after flush
		cacheSizeAfter := store.GetCacheSize()
		
		// Log flushing results
		logger.Info("Cache flushed successfully", 
			"freed_bytes", cacheSizeBefore - cacheSizeAfter,
			"cache_size_before", cacheSizeBefore,
			"cache_size_after", cacheSizeAfter)
		
		// Log the history after flush, unless quiet mode is on
		if !quiet {
			logger.Info("History after flush:")
			if err := store.LogCompleteHistory(config.HistoryOptions{}); err != nil {
				logger.Error("Failed to dump history after flush", "error", err)
			}
		}
		
		return nil
	},
}

func init() {
	// Set up flags for this command
	flushCmd.Flags().BoolVar(&quiet, "quiet", false, "Don't display history before and after flush")
}