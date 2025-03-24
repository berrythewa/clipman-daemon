package cmd

import (
	"time"

	"github.com/berrythewa/clipman-daemon/internal/broker"
	"github.com/berrythewa/clipman-daemon/internal/clipboard"
	"github.com/berrythewa/clipman-daemon/internal/storage"
	"github.com/berrythewa/clipman-daemon/internal/broker"
	"github.com/spf13/cobra"
)

var (
	duration time.Duration
	maxSize  int64
	noBroker bool
)

// Initialize MQTT client if configured and not explicitly disabled
var mqttClient broker.MQTTClientInterface

// runCmd represents the run command
var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the clipboard monitor daemon",
	Long: `Run the clipboard monitor daemon which watches for clipboard changes
and stores them in the history database.

You can specify a duration for testing purposes, otherwise it will run
until interrupted.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		logger.Info("Starting Clipman daemon", "mode", "run")
		
		// If maxSize is specified, override config
		if maxSize > 0 {
			cfg.Storage.MaxSize = maxSize
		}

		// Get all system paths
		paths := cfg.GetPaths()
		
		// Initialize MQTT client if configured and not explicitly disabled
		var mqttClient broker.MQTTClientInterface
		if !noBroker && cfg.Broker.URL != "" {
			logger.Info("Initializing broker connection", 
				"url", cfg.Broker.URL,
				"device_id", cfg.DeviceID)
			
			var err error
			mqttClient, err = broker.NewMQTTClient(cfg, logger)
			if err != nil {
				logger.Warn("Failed to initialize MQTT client", "error", err)
				logger.Info("Continuing without MQTT support")
			} else {
				logger.Info("MQTT client initialized successfully")
			}
		} else {
			if noBroker {
				logger.Info("MQTT broker disabled by command line flag")
			} else if cfg.Broker.URL == "" {
				logger.Info("No MQTT broker URL configured, running without broker connection")
			}
		}
		
		// Initialize storage
		storageConfig := storage.StorageConfig{
			DBPath:     paths.DBFile,
			MaxSize:    cfg.Storage.MaxSize,
			DeviceID:   cfg.DeviceID,
			Logger:     logger,
			MQTTClient: mqttClient,
		}
		
		logger.Info("Storage configuration", 
			"db_path", paths.DBFile,
			"max_size_bytes", storageConfig.MaxSize,
			"device_id", cfg.DeviceID)
			
		store, err := storage.NewBoltStorage(storageConfig)
		if err != nil {
			logger.Error("Failed to initialize storage", "error", err)
			return err
		}
		defer store.Close()
		
		// Start the monitor
		monitor := clipboard.NewMonitor(cfg, mqttClient, logger, store)
		if err := monitor.Start(); err != nil {
			logger.Error("Failed to start monitor", "error", err)
			return err
		}
		
		logger.Info("Monitor started")
		
		if duration > 0 {
			// Run for specified duration
			logger.Info("Running for test duration", "duration", duration)
			time.Sleep(duration)
			
			// Log the complete history after the test duration
			logger.Info("Test complete, logging clipboard history")
			if err := store.LogCompleteHistory(cfg.History); err != nil {
				logger.Error("Failed to log history", "error", err)
			}
			
			logger.Info("Stopping monitor")
			monitor.Stop()
			
			// Properly close all connections
			if mqttClient != nil {
				logger.Info("Disconnecting MQTT client")
				mqttClient.Disconnect()
			}
			
			// Flush logger to ensure all logs are written
			logger.Flush()
			
			// Show recent items
			recentHistory := monitor.GetHistory(10)
			for _, item := range recentHistory {
				dataPreview := "binary data"
				if item.Content.Type == "text" || item.Content.Type == "url" {
					previewLength := min(len(item.Content.Data), 50)
					dataPreview = string(item.Content.Data[:previewLength])
				}
				
				logger.Info("Recent clipboard item",
					"type", item.Content.Type,
					"time", item.Time.Format(time.RFC3339),
					"data_length", len(item.Content.Data),
					"preview", dataPreview)
			}
		} else {
			// Run indefinitely - block until interrupted
			logger.Info("Running until interrupted, press Ctrl+C to stop")
			select {}
		}
		
		return nil
	},
}

func init() {
	// Set up flags for this command
	runCmd.Flags().DurationVarP(&duration, "duration", "d", 0, "Run for a specific duration (for testing)")
	runCmd.Flags().Int64Var(&maxSize, "max-size", 0, "Override max cache size in bytes (default 100MB)")
	runCmd.Flags().BoolVar(&noBroker, "no-broker", false, "Disable MQTT broker connection even if configured")
}

// Helper function to get minimum of two values
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
} 