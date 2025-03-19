package main

import (
	"flag"
	"log"
	"path/filepath"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/clipboard"
	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/storage"
	"github.com/berrythewa/clipman-daemon/pkg/utils"
)

func main() {
	// Parse command line flags
	logLevel := flag.String("log-level", "", "Log level (debug, info, warn, error)")
	testDuration := flag.Duration("duration", 60*time.Second, "How long to run the test")
	flag.Parse()

	// Load config first
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	// Override log level if provided via command line
	if *logLevel != "" {
		cfg.LogLevel = *logLevel
	} else if cfg.LogLevel == "" {
		// Default to info level
		cfg.LogLevel = "info"
	}

	logger := utils.NewLogger(cfg.LogLevel, nil)
	logger.Info("Starting Clipman daemon", "log_level", cfg.LogLevel)

	// Use DataDir for storage
	dbPath := filepath.Join(cfg.DataDir, "clipboard.db")
	storageConfig := storage.StorageConfig{
		DBPath:   dbPath,
		MaxSize:  100 * 1024 * 1024, // 100MB default
		DeviceID: cfg.DeviceID,
		Logger:   logger,
	}
	
	store, err := storage.NewBoltStorage(storageConfig)
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	monitor := clipboard.NewMonitor(cfg, nil, logger, store)
	if err := monitor.Start(); err != nil {
		log.Fatal(err)
	}

	logger.Info("Monitor started, running for test duration", "duration", *testDuration)
	time.Sleep(*testDuration)
	
	logger.Info("Test complete, stopping monitor")
	monitor.Stop()

	history := monitor.GetHistory(10)
	for _, item := range history {
		logger.Info("Clipboard item",
			"type", item.Content.Type,
			"time", item.Time.Format(time.RFC3339),
			"data_length", len(item.Content.Data),
			"preview", string(item.Content.Data[:utils.Min(len(item.Content.Data), 50)]))
	}
}