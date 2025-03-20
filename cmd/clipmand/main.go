package main

import (
	"flag"
	"log"
	"path/filepath"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/clipboard"
	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/storage"
	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/berrythewa/clipman-daemon/pkg/utils"
)

// Command modes - which mode to run in
type commandMode int

const (
	modeRun commandMode = iota
	modeHistory
	modeDumpHistory
	modeFlushCache
)

func main() {
	// Define command line flags
	// Mode selection flags
	flagRun := flag.Bool("run", false, "Run the clipboard monitor (default if no other mode specified)")
	flagHistory := flag.Bool("history", false, "View and filter clipboard history")
	flagDumpHistory := flag.Bool("dump-history", false, "Dump complete clipboard history")
	flagFlushCache := flag.Bool("flush-cache", false, "Force flush the clipboard cache")
	
	// General configuration flags
	flagLogLevel := flag.String("log-level", "", "Log level (debug, info, warn, error)")
	flagMaxSize := flag.Int64("max-size", 0, "Override max cache size in bytes (default 100MB)")
	flagDuration := flag.Duration("duration", 60*time.Second, "How long to run in test mode")
	flagDeviceID := flag.String("device-id", "", "Override device ID")
	
	// History filtering flags
	flagLimit := flag.Int("limit", 0, "Maximum number of history entries to retrieve (0 for all)")
	flagSince := flag.String("since", "", "Retrieve history since this time (RFC3339 format)")
	flagBefore := flag.String("before", "", "Retrieve history before this time (RFC3339 format)")
	flagType := flag.String("type", "", "Filter history by content type (text, image, url, file, filepath)")
	flagReverse := flag.Bool("reverse", false, "Reverse history order (newest first)")
	flagMinSize := flag.Int("min-size", 0, "Minimum content size in bytes")
	flagContentMaxSize := flag.Int("content-max-size", 0, "Maximum content size in bytes")
	
	flag.Parse()
	
	// Determine the run mode based on flags
	mode := modeRun // Default mode
	if *flagHistory {
		mode = modeHistory
	} else if *flagDumpHistory {
		mode = modeDumpHistory
	} else if *flagFlushCache {
		mode = modeFlushCache
	} else if *flagRun {
		mode = modeRun
	}
	
	// Load config
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}
	
	// Override config with command line flags
	if *flagLogLevel != "" {
		cfg.LogLevel = *flagLogLevel
	}
	
	if *flagDeviceID != "" {
		cfg.DeviceID = *flagDeviceID
	}
	
	// Configure history options
	if *flagLimit > 0 {
		cfg.History.Limit = *flagLimit
	}
	
	if *flagSince != "" {
		sinceTime, err := time.Parse(time.RFC3339, *flagSince)
		if err != nil {
			log.Fatalf("Invalid time format for --since: %v", err)
		}
		cfg.History.Since = sinceTime
	}
	
	if *flagBefore != "" {
		beforeTime, err := time.Parse(time.RFC3339, *flagBefore)
		if err != nil {
			log.Fatalf("Invalid time format for --before: %v", err)
		}
		cfg.History.Before = beforeTime
	}
	
	if *flagType != "" {
		switch *flagType {
		case "text":
			cfg.History.ContentType = types.TypeText
		case "image":
			cfg.History.ContentType = types.TypeImage
		case "url":
			cfg.History.ContentType = types.TypeURL
		case "file":
			cfg.History.ContentType = types.TypeFile
		case "filepath":
			cfg.History.ContentType = types.TypeFilePath
		default:
			log.Fatalf("Invalid content type: %s", *flagType)
		}
	}
	
	cfg.History.Reverse = *flagReverse
	cfg.History.MinSize = *flagMinSize
	if *flagContentMaxSize > 0 {
		cfg.History.MaxSize = *flagContentMaxSize
	}
	
	// Create logger
	logger := utils.NewLogger(cfg.LogLevel, nil)
	logger.Info("Starting Clipman daemon", "log_level", cfg.LogLevel, "mode", getModeString(mode))
	
	// Initialize storage
	dbPath := filepath.Join(cfg.DataDir, "clipboard.db")
	storageConfig := storage.StorageConfig{
		DBPath:   dbPath,
		MaxSize:  *flagMaxSize, // This is the correct type (int64)
		DeviceID: cfg.DeviceID,
		Logger:   logger,
	}
	
	// Use default size if not specified
	if storageConfig.MaxSize == 0 {
		storageConfig.MaxSize = 100 * 1024 * 1024 // 100MB default
	}
	
	logger.Info("Storage configuration", 
		"db_path", dbPath,
		"max_size_bytes", storageConfig.MaxSize,
		"device_id", cfg.DeviceID)
	
	store, err := storage.NewBoltStorage(storageConfig)
	if err != nil {
		log.Fatalf("Failed to initialize storage: %v", err)
	}
	defer store.Close()
	
	// Execute the appropriate mode
	switch mode {
	case modeHistory:
		handleHistoryMode(cfg, store, logger)
	case modeDumpHistory:
		handleDumpHistoryMode(cfg, store, logger)
	case modeFlushCache:
		handleFlushCacheMode(cfg, store, logger)
	case modeRun:
		handleRunMode(cfg, store, logger, *flagDuration)
	}
}

// Helper functions for different command modes

func handleHistoryMode(cfg *config.Config, store *storage.BoltStorage, logger *utils.Logger) {
	logger.Info("Retrieving filtered clipboard history")
	if err := store.LogCompleteHistory(cfg.History); err != nil {
		logger.Error("Failed to retrieve history", "error", err)
	}
}

func handleDumpHistoryMode(cfg *config.Config, store *storage.BoltStorage, logger *utils.Logger) {
	logger.Info("Dumping complete clipboard history")
	// Use default options for a complete dump
	if err := store.LogCompleteHistory(config.HistoryOptions{}); err != nil {
		logger.Error("Failed to dump history", "error", err)
	}
}

func handleFlushCacheMode(cfg *config.Config, store *storage.BoltStorage, logger *utils.Logger) {
	logger.Info("Forcing clipboard cache flush")
	
	// First log the history before flush
	logger.Info("History before flush:")
	if err := store.LogCompleteHistory(config.HistoryOptions{}); err != nil {
		logger.Error("Failed to dump history before flush", "error", err)
	}
	
	// Now flush the cache
	if err := store.FlushCache(); err != nil {
		logger.Error("Failed to flush cache", "error", err)
		return
	}
	
	// Log the history after flush
	logger.Info("History after flush:")
	if err := store.LogCompleteHistory(config.HistoryOptions{}); err != nil {
		logger.Error("Failed to dump history after flush", "error", err)
	}
}

func handleRunMode(cfg *config.Config, store *storage.BoltStorage, logger *utils.Logger, duration time.Duration) {
	monitor := clipboard.NewMonitor(cfg, nil, logger, store)
	if err := monitor.Start(); err != nil {
		logger.Error("Failed to start monitor", "error", err)
		return
	}
	
	logger.Info("Monitor started, running for test duration", "duration", duration)
	time.Sleep(duration)
	
	// Log the complete history after the test duration
	logger.Info("Test complete, logging clipboard history")
	if err := store.LogCompleteHistory(config.HistoryOptions{}); err != nil {
		logger.Error("Failed to log history", "error", err)
	}
	
	logger.Info("Stopping monitor")
	monitor.Stop()
	
	// Show recent items
	recentHistory := monitor.GetHistory(10)
	for _, item := range recentHistory {
		logger.Info("Recent clipboard item",
			"type", item.Content.Type,
			"time", item.Time.Format(time.RFC3339),
			"data_length", len(item.Content.Data),
			"preview", string(item.Content.Data[:utils.Min(len(item.Content.Data), 50)]))
	}
}

// Helper function to get string representation of mode
func getModeString(mode commandMode) string {
	switch mode {
	case modeRun:
		return "run"
	case modeHistory:
		return "history"
	case modeDumpHistory:
		return "dump-history"
	case modeFlushCache:
		return "flush-cache"
	default:
		return "unknown"
	}
}