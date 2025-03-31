// Package sync provides synchronization capabilities between Clipman instances
package sync

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/storage"
	"github.com/berrythewa/clipman-daemon/internal/types"
	"go.uber.org/zap"
)

// SyncDaemon represents a persistent sync client daemon
type SyncDaemon struct {
	client       SyncClient
	cfg          *config.Config
	logger       *zap.Logger
	sockPath     string
	listener     net.Listener
	mu           sync.Mutex
	isRunning    bool
	stopCh       chan struct{}
}

// Command represents an IPC command sent to the daemon
type Command struct {
	Action  string            `json:"action"`  // "join", "leave", "list", etc.
	Groups  []string          `json:"groups"`  // Group names for operations
	Options map[string]string `json:"options"` // Additional options
}

// Response represents an IPC response from the daemon
type Response struct {
	Success bool     `json:"success"`
	Message string   `json:"message"`
	Data    any      `json:"data"`
	Groups  []string `json:"groups,omitempty"`
	Errors  []string `json:"errors,omitempty"`
}

// NewSyncDaemon creates a new persistent sync daemon
func NewSyncDaemon(cfg *config.Config, logger *zap.Logger) (*SyncDaemon, error) {
	// Create socket path
	sockDir := filepath.Join(cfg.SystemPaths.DataDir, "sockets")
	if err := os.MkdirAll(sockDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create socket directory: %w", err)
	}
	
	sockPath := filepath.Join(sockDir, "clipman-sync.sock")
	
	// Create sync client
	client, err := CreateClient(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create sync client: %w", err)
	}
	
	return &SyncDaemon{
		client:    client,
		cfg:       cfg,
		logger:    logger,
		sockPath:  sockPath,
		stopCh:    make(chan struct{}),
	}, nil
}

// Start starts the sync daemon
func (d *SyncDaemon) Start() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	if d.isRunning {
		return nil // Already running
	}
	
	// Remove existing socket if any
	if _, err := os.Stat(d.sockPath); err == nil {
		if err := os.Remove(d.sockPath); err != nil {
			return fmt.Errorf("failed to remove existing socket: %w", err)
		}
	}
	
	// Create Unix domain socket
	listener, err := net.Listen("unix", d.sockPath)
	if err != nil {
		return fmt.Errorf("failed to create socket: %w", err)
	}
	d.listener = listener
	
	// Connect the client
	if err := d.client.Connect(); err != nil {
		listener.Close()
		return fmt.Errorf("failed to connect sync client: %w", err)
	}
	
	// Start the IPC server
	go d.serve()
	
	d.isRunning = true
	d.logger.Info("Sync daemon started", zap.String("socket", d.sockPath))
	
	return nil
}

// Stop stops the sync daemon
func (d *SyncDaemon) Stop() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	
	if !d.isRunning {
		return nil // Not running
	}
	
	// Signal the server to stop
	close(d.stopCh)
	
	// Close the listener
	if d.listener != nil {
		d.listener.Close()
	}
	
	// Disconnect the client
	d.client.Disconnect()
	
	// Remove the socket file
	os.Remove(d.sockPath)
	
	d.isRunning = false
	d.logger.Info("Sync daemon stopped")
	
	return nil
}

// serve handles incoming IPC connections
func (d *SyncDaemon) serve() {
	for {
		// Check if we need to stop
		select {
		case <-d.stopCh:
			return
		default:
			// Continue accepting connections
		}
		
		// Accept a connection
		conn, err := d.listener.Accept()
		if err != nil {
			select {
			case <-d.stopCh:
				return // Normal shutdown
			default:
				d.logger.Error("Failed to accept connection", zap.Error(err))
				continue
			}
		}
		
		// Handle the connection in a goroutine
		go d.handleConnection(conn)
	}
}

// handleConnection processes an IPC connection
func (d *SyncDaemon) handleConnection(conn net.Conn) {
	defer conn.Close()
	
	// Decode the command
	var cmd Command
	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(&cmd); err != nil {
		d.logger.Error("Failed to decode command", zap.Error(err))
		sendErrorResponse(conn, "Failed to decode command")
		return
	}
	
	// Process the command
	var response Response
	
	switch cmd.Action {
	case "join":
		response = d.handleJoinCommand(cmd)
	case "leave":
		response = d.handleLeaveCommand(cmd)
	case "list":
		response = d.handleListCommand(cmd)
	case "status":
		response = d.handleStatusCommand(cmd)
	case "resync":
		response = d.handleResyncCommand(cmd)
	default:
		response = Response{
			Success: false,
			Message: fmt.Sprintf("Unknown action: %s", cmd.Action),
		}
	}
	
	// Send the response
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(response); err != nil {
		d.logger.Error("Failed to encode response", zap.Error(err))
	}
}

// handleJoinCommand processes a join command
func (d *SyncDaemon) handleJoinCommand(cmd Command) Response {
	if len(cmd.Groups) == 0 {
		return Response{
			Success: false,
			Message: "No groups specified",
		}
	}
	
	var errors []string
	var success []string
	
	for _, group := range cmd.Groups {
		if err := d.client.JoinGroup(group); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", group, err))
		} else {
			success = append(success, group)
		}
	}
	
	if len(success) > 0 {
		// Update default group if needed
		if d.cfg.Sync.DefaultGroup == "" && len(success) > 0 {
			d.cfg.Sync.DefaultGroup = success[0]
			if err := d.cfg.Save(); err != nil {
				d.logger.Error("Failed to save config", zap.Error(err))
			}
		}
		
		return Response{
			Success: true,
			Message: fmt.Sprintf("Joined %d groups", len(success)),
			Groups:  success,
			Errors:  errors,
		}
	}
	
	return Response{
		Success: false,
		Message: "Failed to join any groups",
		Errors:  errors,
	}
}

// handleLeaveCommand processes a leave command
func (d *SyncDaemon) handleLeaveCommand(cmd Command) Response {
	if len(cmd.Groups) == 0 {
		return Response{
			Success: false,
			Message: "No groups specified",
		}
	}
	
	var errors []string
	var success []string
	
	for _, group := range cmd.Groups {
		if err := d.client.LeaveGroup(group); err != nil {
			errors = append(errors, fmt.Sprintf("%s: %v", group, err))
		} else {
			success = append(success, group)
			
			// If this was the default group, clear it
			if d.cfg.Sync.DefaultGroup == group {
				d.cfg.Sync.DefaultGroup = ""
				if err := d.cfg.Save(); err != nil {
					d.logger.Error("Failed to save config", zap.Error(err))
				}
			}
		}
	}
	
	if len(success) > 0 {
		return Response{
			Success: true,
			Message: fmt.Sprintf("Left %d groups", len(success)),
			Groups:  success,
			Errors:  errors,
		}
	}
	
	return Response{
		Success: false,
		Message: "Failed to leave any groups",
		Errors:  errors,
	}
}

// handleListCommand processes a list command
func (d *SyncDaemon) handleListCommand(cmd Command) Response {
	groups, err := d.client.ListGroups()
	if err != nil {
		return Response{
			Success: false,
			Message: "Failed to list groups",
			Errors:  []string{err.Error()},
		}
	}
	
	return Response{
		Success: true,
		Message: fmt.Sprintf("Found %d groups", len(groups)),
		Groups:  groups,
	}
}

// handleStatusCommand processes a status command
func (d *SyncDaemon) handleStatusCommand(cmd Command) Response {
	connected := d.client.IsConnected()
	
	status := map[string]interface{}{
		"connected":     connected,
		"mode":          d.cfg.Sync.Mode,
		"default_group": d.cfg.Sync.DefaultGroup,
	}
	
	// Get groups if connected
	var groups []string
	var err error
	if connected {
		groups, err = d.client.ListGroups()
		if err != nil {
			return Response{
				Success: false,
				Message: "Failed to get status",
				Errors:  []string{err.Error()},
			}
		}
	}
	
	return Response{
		Success: true,
		Message: "Sync status retrieved",
		Data:    status,
		Groups:  groups,
	}
}

// handleResyncCommand processes a resync command
func (d *SyncDaemon) handleResyncCommand(cmd Command) Response {
	// Create a storage instance to access clipboard history
	storageConfig := storage.StorageConfig{
		DBPath:   d.cfg.Storage.DBPath,
		MaxSize:  d.cfg.Storage.MaxSize,
		DeviceID: d.cfg.DeviceID,
		Logger:   d.logger,
	}
	
	// Create the storage
	store, err := storage.NewBoltStorage(storageConfig)
	if err != nil {
		return Response{
			Success: false,
			Message: fmt.Sprintf("Failed to open storage: %v", err),
		}
	}
	defer store.Close()
	
	// Get all clipboard content
	timeZero := time.Time{} // Unix epoch 0
	contents, err := store.GetContentSince(timeZero)
	if err != nil {
		return Response{
			Success: false,
			Message: fmt.Sprintf("Failed to get content history: %v", err),
		}
	}
	
	// Create cache message
	cache := &types.CacheMessage{
		DeviceID:    d.cfg.DeviceID,
		ContentList: contents,
		TotalSize:   store.GetCacheSize(),
		Timestamp:   time.Now(),
	}
	
	// Publish directly using the daemon's client
	if err := d.client.PublishCache(cache); err != nil {
		return Response{
			Success: false,
			Message: fmt.Sprintf("Failed to publish cache history: %v", err),
		}
	}
	
	return Response{
		Success: true,
		Message: fmt.Sprintf("Successfully resynced clipboard history (%d items)", len(contents)),
	}
}

// sendErrorResponse sends an error response
func sendErrorResponse(conn net.Conn, msg string) {
	response := Response{
		Success: false,
		Message: msg,
	}
	
	encoder := json.NewEncoder(conn)
	encoder.Encode(response)
} 