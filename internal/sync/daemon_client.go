package sync

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/config"
)

// DaemonClient provides a client interface to communicate with the sync daemon
type DaemonClient struct {
	sockPath string
}

// NewDaemonClient creates a new client for the sync daemon
func NewDaemonClient(cfg *config.Config) *DaemonClient {
	sockPath := filepath.Join(cfg.SystemPaths.DataDir, "sockets", "clipman-sync.sock")
	
	return &DaemonClient{
		sockPath: sockPath,
	}
}

// IsDaemonRunning checks if the sync daemon is running
func (c *DaemonClient) IsDaemonRunning() bool {
	if _, err := os.Stat(c.sockPath); err != nil {
		return false
	}
	
	// Try to connect to verify it's active
	conn, err := net.DialTimeout("unix", c.sockPath, 100*time.Millisecond)
	if err != nil {
		// Socket exists but can't connect, might be stale
		return false
	}
	conn.Close()
	
	return true
}

// sendCommand sends a command to the daemon and returns the response
func (c *DaemonClient) sendCommand(cmd Command) (*Response, error) {
	// Check if daemon is running
	if !c.IsDaemonRunning() {
		return nil, fmt.Errorf("sync daemon is not running")
	}
	
	// Connect to the daemon
	conn, err := net.Dial("unix", c.sockPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to sync daemon: %w", err)
	}
	defer conn.Close()
	
	// Send the command
	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(cmd); err != nil {
		return nil, fmt.Errorf("failed to send command: %w", err)
	}
	
	// Read the response
	var resp Response
	decoder := json.NewDecoder(conn)
	if err := decoder.Decode(&resp); err != nil {
		return nil, fmt.Errorf("failed to receive response: %w", err)
	}
	
	return &resp, nil
}

// JoinGroups joins one or more synchronization groups
func (c *DaemonClient) JoinGroups(groups []string) (*Response, error) {
	cmd := Command{
		Action: "join",
		Groups: groups,
	}
	
	return c.sendCommand(cmd)
}

// LeaveGroups leaves one or more synchronization groups
func (c *DaemonClient) LeaveGroups(groups []string) (*Response, error) {
	cmd := Command{
		Action: "leave",
		Groups: groups,
	}
	
	return c.sendCommand(cmd)
}

// ListGroups lists all joined synchronization groups
func (c *DaemonClient) ListGroups() (*Response, error) {
	cmd := Command{
		Action: "list",
	}
	
	return c.sendCommand(cmd)
}

// GetStatus gets the current sync status
func (c *DaemonClient) GetStatus() (*Response, error) {
	cmd := Command{
		Action: "status",
	}
	
	return c.sendCommand(cmd)
}

// Resync resynchronizes the clipboard history with other devices
func (c *DaemonClient) Resync() (*Response, error) {
	cmd := Command{
		Action: "resync",
	}
	
	return c.sendCommand(cmd)
} 