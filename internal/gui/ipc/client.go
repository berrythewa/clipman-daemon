package ipc

import (
	"fmt"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/ipc"
	"github.com/berrythewa/clipman-daemon/internal/types"
)

const (
	maxRetries = 3
	retryDelay = 500 * time.Millisecond
)

// Client provides an interface for GUI-daemon communication
type Client struct {
	socketPath string
}

// NewClient creates a new IPC client
func NewClient(socketPath string) *Client {
	return &Client{
		socketPath: socketPath,
	}
}

// sendRequestWithRetry sends a request with retry logic
func (c *Client) sendRequestWithRetry(req *ipc.Request) (*ipc.Response, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		resp, err := ipc.SendRequest(c.socketPath, req)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		time.Sleep(retryDelay)
	}
	return nil, fmt.Errorf("failed after %d retries: %w", maxRetries, lastErr)
}

// GetHistory retrieves the clipboard history
func (c *Client) GetHistory(limit int) ([]*types.ClipboardContent, error) {
	req := &ipc.Request{
		Command: "history",
		Args: map[string]interface{}{
			"limit": limit,
		},
	}

	resp, err := c.sendRequestWithRetry(req)
	if err != nil {
		return nil, err
	}

	if resp.Status != "ok" {
		return nil, fmt.Errorf("failed to get history: %s", resp.Message)
	}

	history, ok := resp.Data.([]*types.ClipboardContent)
	if !ok {
		return nil, fmt.Errorf("invalid history data type")
	}

	return history, nil
}

// WriteToClipboard writes content to the clipboard
func (c *Client) WriteToClipboard(content *types.ClipboardContent) error {
	req := &ipc.Request{
		Command: "write",
		Args: map[string]interface{}{
			"content": content,
		},
	}

	resp, err := c.sendRequestWithRetry(req)
	if err != nil {
		return err
	}

	if resp.Status != "ok" {
		return fmt.Errorf("failed to write to clipboard: %s", resp.Message)
	}

	return nil
}

// ClearHistory clears the clipboard history
func (c *Client) ClearHistory() error {
	req := &ipc.Request{
		Command: "flush",
	}

	resp, err := c.sendRequestWithRetry(req)
	if err != nil {
		return err
	}

	if resp.Status != "ok" {
		return fmt.Errorf("failed to clear history: %s", resp.Message)
	}

	return nil
}

// GetSyncStatus gets the current sync status
func (c *Client) GetSyncStatus() (*types.SyncStatus, error) {
	req := &ipc.Request{
		Command: "sync_status",
	}

	resp, err := c.sendRequestWithRetry(req)
	if err != nil {
		return nil, err
	}

	if resp.Status != "ok" {
		return nil, fmt.Errorf("failed to get sync status: %s", resp.Message)
	}

	status, ok := resp.Data.(*types.SyncStatus)
	if !ok {
		return nil, fmt.Errorf("invalid sync status data type")
	}

	return status, nil
}

// GetPairedDevices gets the list of paired devices
func (c *Client) GetPairedDevices() ([]types.PairedDevice, error) {
	req := &ipc.Request{
		Command: "paired_devices",
	}

	resp, err := c.sendRequestWithRetry(req)
	if err != nil {
		return nil, err
	}

	if resp.Status != "ok" {
		return nil, fmt.Errorf("failed to get paired devices: %s", resp.Message)
	}

	devices, ok := resp.Data.([]types.PairedDevice)
	if !ok {
		return nil, fmt.Errorf("invalid paired devices data type")
	}

	return devices, nil
}

// StartPairing starts the device pairing process
func (c *Client) StartPairing() error {
	req := &ipc.Request{
		Command: "start_pairing",
	}

	resp, err := c.sendRequestWithRetry(req)
	if err != nil {
		return err
	}

	if resp.Status != "ok" {
		return fmt.Errorf("failed to start pairing: %s", resp.Message)
	}

	return nil
}

// StopPairing stops the device pairing process
func (c *Client) StopPairing() error {
	req := &ipc.Request{
		Command: "stop_pairing",
	}

	resp, err := c.sendRequestWithRetry(req)
	if err != nil {
		return err
	}

	if resp.Status != "ok" {
		return fmt.Errorf("failed to stop pairing: %s", resp.Message)
	}

	return nil
}

// UnpairDevice removes a paired device
func (c *Client) UnpairDevice(deviceID string) error {
	req := &ipc.Request{
		Command: "unpair_device",
		Args: map[string]interface{}{
			"device_id": deviceID,
		},
	}

	resp, err := c.sendRequestWithRetry(req)
	if err != nil {
		return err
	}

	if resp.Status != "ok" {
		return fmt.Errorf("failed to unpair device: %s", resp.Message)
	}

	return nil
} 