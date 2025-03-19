package main

import (
	"log"
	"path/filepath"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/clipboard"
	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/storage"
	"github.com/berrythewa/clipman-daemon/pkg/utils"
)

func main() {
	// Load config first
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	logger := utils.NewLogger(cfg.LogLevel, nil)

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

	time.Sleep(time.Minute)
	monitor.Stop()

	history := monitor.GetHistory(10)
	for _, item := range history {
		logger.Info("Clipboard item",
			"type", item.Content.Type,
			"time", item.Time,
			"data", string(item.Content.Data))
	}
}
