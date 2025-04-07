package sync

import (
	"testing"
	"time"

	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

// TestNewSyncManager tests the creation of a new SyncManager
func TestNewSyncManager(t *testing.T) {
	// Create a test configuration
	cfg := &config.Config{
		Sync: config.SyncConfig{
			Enabled:         true,
			Protocol:        "mqtt",
			URL:             "mqtt://localhost:1883",
			ClientID:        "test-client",
			DefaultGroup:    "test-group",
			EnableDiscovery: true,
			DiscoveryMethod: "mdns",
		},
		DeviceName: "test-device",
	}

	// Create a test logger
	logger, _ := zap.NewDevelopment()

	// Create a new SyncManager
	syncManager, err := NewSyncManager(cfg, logger)
	
	// Verify the SyncManager was created successfully
	assert.NoError(t, err)
	assert.NotNil(t, syncManager)
	
	// Check initial state
	assert.False(t, syncManager.IsConnected())
	
	// Verify the status is correctly initialized
	status := syncManager.Status()
	assert.False(t, status.Connected)
	assert.Equal(t, "mqtt", status.Mode)
	assert.Empty(t, status.ActiveGroups)
	assert.Equal(t, 0, status.PeerCount)
}

// TestSyncManagerContentHandling tests content handler registration
func TestSyncManagerContentHandling(t *testing.T) {
	// Create a minimal configuration for testing
	cfg := &config.Config{
		Sync: config.SyncConfig{
			Enabled:  true,
			Protocol: "mqtt",
		},
	}

	// Create a test logger
	logger, _ := zap.NewDevelopment()

	// Create a new SyncManager
	syncManager, err := NewSyncManager(cfg, logger)
	assert.NoError(t, err)
	assert.NotNil(t, syncManager)
	
	// Test handler registration
	handlerCalled := false
	handlerContent := (*types.ClipboardContent)(nil)
	handlerPeer := PeerInfo{}
	
	syncManager.SetContentHandler(func(content *types.ClipboardContent, peer PeerInfo) {
		handlerCalled = true
		handlerContent = content
		handlerPeer = peer
	})
	
	// Simulate receiving content by directly calling the handler
	// This is a bit of a hack, but allows testing the handler without a real connection
	testContent := &types.ClipboardContent{
		Type: types.TypeText,
		Data: []byte("Test content"),
	}
	
	testPeer := PeerInfo{
		ID:   "test-peer",
		Name: "Test Peer",
	}
	
	// Access the internal slice of handlers
	manager := syncManager.(*Manager)
	assert.NotEmpty(t, manager.contentHandlers)
	
	// Call the first handler directly
	manager.contentHandlers[0](testContent, testPeer)
	
	// Verify the handler was called with the correct arguments
	assert.True(t, handlerCalled)
	assert.Equal(t, testContent, handlerContent)
	assert.Equal(t, testPeer, handlerPeer)
}

// MockProtocolClient is a mock implementation of the Protocol interface for testing
type MockProtocolClient struct {
	connected     bool
	messageHandler func(Message)
	groups        []string
}

func (m *MockProtocolClient) Initialize() error { return nil }
func (m *MockProtocolClient) Start() error { return nil }
func (m *MockProtocolClient) Stop() error { return nil }
func (m *MockProtocolClient) IsConnected() bool { return m.connected }
func (m *MockProtocolClient) Connect() error { m.connected = true; return nil }
func (m *MockProtocolClient) Disconnect() error { m.connected = false; return nil }
func (m *MockProtocolClient) Send(msg Message) error { return nil }
func (m *MockProtocolClient) SetMessageHandler(handler func(Message)) { m.messageHandler = handler }
func (m *MockProtocolClient) AddHandler(handler func(Message)) { m.messageHandler = handler }
func (m *MockProtocolClient) JoinGroup(group string) error { 
	m.groups = append(m.groups, group)
	return nil 
}
func (m *MockProtocolClient) LeaveGroup(group string) error {
	for i, g := range m.groups {
		if g == group {
			m.groups = append(m.groups[:i], m.groups[i+1:]...)
			break
		}
	}
	return nil
}
func (m *MockProtocolClient) ListGroups() []string { return m.groups }

// TestSyncManagerGroupOperations tests group management operations
func TestSyncManagerGroupOperations(t *testing.T) {
	// Create a minimal configuration for testing
	cfg := &config.Config{
		Sync: config.SyncConfig{
			Enabled:  true,
			Protocol: "mqtt",
		},
	}

	// Create a test logger
	logger, _ := zap.NewDevelopment()

	// Create a new SyncManager
	syncManager, err := NewSyncManager(cfg, logger)
	assert.NoError(t, err)
	
	// Replace the real client with our mock client
	manager := syncManager.(*Manager)
	mockClient := &MockProtocolClient{connected: true, groups: []string{}}
	manager.client = mockClient
	
	// Update the status to connected
	manager.statusMu.Lock()
	manager.status = SyncStatusConnected
	manager.statusMu.Unlock()
	
	// Test joining a group
	err = syncManager.JoinGroup("test-group")
	assert.NoError(t, err)
	
	// Verify the group was joined
	groups, err := syncManager.ListGroups()
	assert.NoError(t, err)
	assert.Contains(t, groups, "test-group")
	
	// Test leaving a group
	err = syncManager.LeaveGroup("test-group")
	assert.NoError(t, err)
	
	// Verify the group was left
	groups, err = syncManager.ListGroups()
	assert.NoError(t, err)
	assert.NotContains(t, groups, "test-group")
}

// TestSyncManagerConnect tests connection handling
func TestSyncManagerConnect(t *testing.T) {
	// Create a minimal configuration for testing
	cfg := &config.Config{
		Sync: config.SyncConfig{
			Enabled:  true,
			Protocol: "mqtt",
		},
	}

	// Create a test logger
	logger, _ := zap.NewDevelopment()

	// Create a new SyncManager
	syncManager, err := NewSyncManager(cfg, logger)
	assert.NoError(t, err)
	
	// Replace the real client with our mock client
	manager := syncManager.(*Manager)
	mockClient := &MockProtocolClient{connected: false, groups: []string{}}
	manager.client = mockClient
	
	// Test connecting
	err = syncManager.Connect()
	assert.NoError(t, err)
	
	// Verify the connection status
	assert.True(t, syncManager.IsConnected())
	assert.True(t, mockClient.connected)
	
	// Test disconnecting
	err = syncManager.Disconnect()
	assert.NoError(t, err)
	
	// Verify the connection status
	assert.False(t, syncManager.IsConnected())
	assert.False(t, mockClient.connected)
} 