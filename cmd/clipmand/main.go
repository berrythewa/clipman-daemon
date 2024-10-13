package main

import (
	"log"
	"path/filepath"

	"github.com/berrythewa/clipman-daemon/internal/clipboard"
	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/broker"
	"github.com/berrythewa/clipman-daemon/internal/storage"
	"github.com/berrythewa/clipman-daemon/pkg/utils"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	logger := utils.NewLogger(cfg.LogLevel)

	mqttClient, err := broker.NewMQTTClient(cfg, logger)
	if err != nil {
		logger.Fatal("Failed to create MQTT client", "error", err)
	}

	if err := mqttClient.SubscribeToCommands(); err != nil {
		logger.Fatal("Failed to subscribe to commands", "error", err)
	}

	dbPath := filepath.Join(cfg.DataDir, "clipman.db")
	storage, err := storage.NewBoltStorage(dbPath)
	if err != nil {
		logger.Fatal("Failed to create storage", "error", err)
	}
	defer storage.Close()

	monitor := clipboard.NewMonitor(cfg, mqttClient, logger, storage)
	if err := monitor.Start(); err != nil {
		logger.Fatal("Failed to start clipboard monitor", "error", err)
	}

	// Keep the main goroutine alive
	select {}
}