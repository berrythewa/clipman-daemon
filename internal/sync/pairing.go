// Package sync provides clipboard synchronization using libp2p
package sync

import (
	"bufio"
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"
	"github.com/libp2p/go-libp2p/core/peer"
	"github.com/libp2p/go-libp2p/core/protocol"
	"github.com/multiformats/go-multiaddr"
	"go.uber.org/zap"
)

const (
	// PairingRequestTimeout is the timeout for pairing requests
	PairingRequestTimeout = 5 * time.Minute

	// AuthCodeLength is the length of the verification code in bytes
	AuthCodeLength = 4
)

// PairingRequestCallback is the callback for handling pairing requests
type PairingRequestCallback func(request PairingRequest, remotePeerId string) (bool, error)

// PairingRequest contains information about a device requesting pairing
type PairingRequest struct {
	DeviceName string            `json:"device_name"`
	DeviceType string            `json:"device_type"`
	Timestamp  time.Time         `json:"timestamp"`
	RandomData string            `json:"random_data"` // Used for verification code generation
	PeerID     string            `json:"peer_id"`     // Requester's peer ID
	Nonce      string            `json:"nonce"`       // Random nonce for this request
	Metadata   map[string]string `json:"metadata"`    // Additional device metadata
}

// PairingResponse is the response to a pairing request
type PairingResponse struct {
	Accepted      bool              `json:"accepted"`      // Whether the pairing was accepted
	ErrorMessage  string            `json:"error_message"` // Error message if rejected
	PairingCode   string            `json:"pairing_code"`  // Verification code
	DeviceName    string            `json:"device_name"`   // Responder's device name
	DeviceType    string            `json:"device_type"`   // Responder's device type
	Timestamp     time.Time         `json:"timestamp"`     // Time of response
	RandomData    string            `json:"random_data"`   // Used for verification code generation
	PeerID        string            `json:"peer_id"`       // Responder's peer ID
	Metadata      map[string]string `json:"metadata"`      // Additional device metadata
	ValidUntil    time.Time         `json:"valid_until"`   // When this pairing expires (0 = never)
}

// PairedDevice represents a device that has been paired
type PairedDevice struct {
	PeerID      string            `json:"peer_id"`      // Peer ID of the paired device
	DeviceName  string            `json:"device_name"`  // Human-readable name
	DeviceType  string            `json:"device_type"`  // Type of device (desktop, mobile, etc.)
	LastSeen    time.Time         `json:"last_seen"`    // When this device was last seen
	PairedAt    time.Time         `json:"paired_at"`    // When the pairing was established
	Addresses   []string          `json:"addresses"`    // Known addresses for this device
	Metadata    map[string]string `json:"metadata"`     // Additional device metadata
	Capabilities []string         `json:"capabilities"` // Device capabilities
}

// PairingManager implements the pairing protocol
type PairingManager struct {
	// Host and context
	host           host.Host
	ctx            context.Context
	cancel         context.CancelFunc
	logger         *zap.Logger
	config         *SyncConfig
	protocolManager *ProtocolManager

	// Pairing state
	pairingEnabled    bool
	pairingMutex      sync.RWMutex
	incomingHandler   PairingRequestCallback
	pairingTimeout    *time.Timer
	pairedDevices     map[string]PairedDevice
	devicesLock       sync.RWMutex
	pairingInProgress bool
}

// NewPairingManager creates a new pairing manager
func NewPairingManager(ctx context.Context, host host.Host, pm *ProtocolManager, config *SyncConfig, logger *zap.Logger) *PairingManager {
	pairingCtx, cancel := context.WithCancel(ctx)

	manager := &PairingManager{
		host:           host,
		ctx:            pairingCtx,
		cancel:         cancel,
		logger:         logger.With(zap.String("component", "pairing-manager")),
		config:         config,
		protocolManager: pm,
		pairingEnabled: false,
		pairedDevices:  make(map[string]PairedDevice),
	}

	// Create and register the protocol handler
	pm.AddHandler(manager)

	return manager
}

// ID returns the protocol ID
func (pm *PairingManager) ID() protocol.ID {
	return protocol.ID(PairingProtocolPath)
}

// Handle processes incoming pairing streams
func (pm *PairingManager) Handle(stream network.Stream) {
	remotePeer := stream.Conn().RemotePeer()
	remoteAddr := stream.Conn().RemoteMultiaddr()
	
	pm.logger.Debug("Received pairing stream", 
		zap.String("peer_id", remotePeer.String()),
		zap.String("addr", remoteAddr.String()))
	
	// Create a buffered reader/writer for the stream
	reader := bufio.NewReader(stream)
	writer := bufio.NewWriter(stream)
	
	// Read the message type
	msgType, err := reader.ReadString('\n')
	if err != nil {
		pm.logger.Error("Failed to read message type", zap.Error(err))
		stream.Reset()
		return
	}
	
	// Use a defer to reset the stream when we're done
	defer stream.Close()
	
	// Handle different message types
	switch msgType {
	case "REQUEST\n":
		pm.handlePairingRequest(reader, writer, remotePeer)
	case "VERIFY\n":
		pm.handleVerification(reader, writer, remotePeer)
	default:
		pm.logger.Warn("Unknown pairing message type", zap.String("type", msgType))
	}
}

// Start initializes the pairing manager
func (pm *PairingManager) Start() error {
	// Load paired devices from storage if available
	if err := pm.loadPairedDevices(); err != nil {
		pm.logger.Warn("Failed to load paired devices", zap.Error(err))
	}
	
	return nil
}

// Stop shuts down the pairing manager
func (pm *PairingManager) Stop() error {
	pm.pairingMutex.Lock()
	defer pm.pairingMutex.Unlock()
	
	if pm.pairingEnabled {
		if pm.pairingTimeout != nil {
			pm.pairingTimeout.Stop()
			pm.pairingTimeout = nil
		}
		pm.pairingEnabled = false
	}
	
	// Save paired devices
	if err := pm.savePairedDevices(); err != nil {
		pm.logger.Warn("Failed to save paired devices", zap.Error(err))
	}
	
	pm.cancel()
	return nil
}

// EnablePairing enables pairing mode for the device
func (pm *PairingManager) EnablePairing(handler PairingRequestCallback) (string, error) {
	pm.pairingMutex.Lock()
	defer pm.pairingMutex.Unlock()
	
	if pm.pairingEnabled {
		return "", errors.New("pairing already enabled")
	}
	
	// Set up pairing
	pm.incomingHandler = handler
	pm.pairingEnabled = true
	
	// Configure timeout if set
	if pm.config.PairingTimeout > 0 {
		pm.pairingTimeout = time.AfterFunc(time.Duration(pm.config.PairingTimeout)*time.Second, func() {
			pm.DisablePairing()
			pm.logger.Info("Pairing mode timed out")
		})
	}
	
	// Generate a shareable address
	addr := fmt.Sprintf("%s/p2p/%s", pm.host.Addrs()[0].String(), pm.host.ID().String())
	pm.logger.Info("Pairing enabled", zap.String("address", addr))
	
	return addr, nil
}

// DisablePairing disables pairing mode
func (pm *PairingManager) DisablePairing() {
	pm.pairingMutex.Lock()
	defer pm.pairingMutex.Unlock()
	
	if pm.pairingEnabled {
		if pm.pairingTimeout != nil {
			pm.pairingTimeout.Stop()
			pm.pairingTimeout = nil
		}
		pm.pairingEnabled = false
		pm.incomingHandler = nil
		pm.logger.Info("Pairing disabled")
	}
}

// IsPairingEnabled returns whether pairing is enabled
func (pm *PairingManager) IsPairingEnabled() bool {
	pm.pairingMutex.RLock()
	defer pm.pairingMutex.RUnlock()
	
	return pm.pairingEnabled
}

// RequestPairing sends a pairing request to a device
func (pm *PairingManager) RequestPairing(address string) (*PairingResponse, error) {
	// Parse the address
	addrInfo, err := peer.AddrInfoFromString(address)
	if err != nil {
		return nil, fmt.Errorf("invalid address: %w", err)
	}
	
	// Check if this is our own address
	if addrInfo.ID == pm.host.ID() {
		return nil, errors.New("cannot pair with self")
	}
	
	// Mark pairing as in progress
	pm.pairingMutex.Lock()
	if pm.pairingInProgress {
		pm.pairingMutex.Unlock()
		return nil, errors.New("pairing already in progress")
	}
	pm.pairingInProgress = true
	pm.pairingMutex.Unlock()
	
	// Reset pairing in progress when done
	defer func() {
		pm.pairingMutex.Lock()
		pm.pairingInProgress = false
		pm.pairingMutex.Unlock()
	}()
	
	// Add peer to peerstore temporarily
	pm.host.Peerstore().AddAddrs(addrInfo.ID, addrInfo.Addrs, PairingRequestTimeout)
	
	// Create a context with timeout
	ctx, cancel := context.WithTimeout(pm.ctx, 30*time.Second)
	defer cancel()
	
	// Open a stream to the peer
	stream, err := pm.protocolManager.OpenStream(addrInfo.ID, pm.ID())
	if err != nil {
		return nil, fmt.Errorf("failed to open pairing stream: %w", err)
	}
	defer stream.Close()
	
	// Create reader/writer
	reader := bufio.NewReader(stream)
	writer := bufio.NewWriter(stream)
	
	// Generate random data for verification code
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random data: %w", err)
	}
	randomData := base64.StdEncoding.EncodeToString(randomBytes)
	
	// Create request
	request := PairingRequest{
		DeviceName: pm.config.DeviceName,
		DeviceType: pm.config.DeviceType,
		Timestamp:  time.Now(),
		RandomData: randomData,
		PeerID:     pm.host.ID().String(),
		Nonce:      generateNonce(),
		Metadata:   map[string]string{},
	}
	
	// Add metadata
	request.Metadata["version"] = "1.0.0" // Example version
	
	// Marshal the request
	requestJSON, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}
	
	// Send message type and request
	if _, err := writer.WriteString("REQUEST\n"); err != nil {
		return nil, fmt.Errorf("failed to write message type: %w", err)
	}
	
	if _, err := writer.Write(requestJSON); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}
	
	if _, err := writer.WriteString("\n"); err != nil {
		return nil, fmt.Errorf("failed to write newline: %w", err)
	}
	
	if err := writer.Flush(); err != nil {
		return nil, fmt.Errorf("failed to flush writer: %w", err)
	}
	
	// Wait for response
	responseJSON, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}
	
	// Unmarshal the response
	var response PairingResponse
	if err := json.Unmarshal(responseJSON, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}
	
	// If accepted, add to paired devices
	if response.Accepted {
		// Generate verification code
		pairingCode := generateVerificationCode(request.RandomData, response.RandomData)
		
		// Set the pairing code in the response
		response.PairingCode = pairingCode
		
		// Create paired device entry
		device := PairedDevice{
			PeerID:     addrInfo.ID.String(),
			DeviceName: response.DeviceName,
			DeviceType: response.DeviceType,
			LastSeen:   time.Now(),
			PairedAt:   time.Now(),
			Addresses:  []string{},
			Metadata:   response.Metadata,
		}
		
		// Add addresses
		for _, addr := range addrInfo.Addrs {
			device.Addresses = append(device.Addresses, addr.String())
		}
		
		// Add capabilities if present in metadata
		if caps, ok := response.Metadata["capabilities"]; ok {
			device.Capabilities = []string{caps}
		}
		
		// Add to paired devices
		pm.devicesLock.Lock()
		pm.pairedDevices[addrInfo.ID.String()] = device
		pm.devicesLock.Unlock()
		
		// Save paired devices
		if err := pm.savePairedDevices(); err != nil {
			pm.logger.Warn("Failed to save paired devices", zap.Error(err))
		}
		
		pm.logger.Info("Successfully paired with device",
			zap.String("peer_id", addrInfo.ID.String()),
			zap.String("device_name", response.DeviceName))
		
		// Add the device to the discovery manager for reusable connections
		discoveryService, err := pm.getDiscoveryService()
		if err == nil {
			if err := discoveryService.AddPeer(address); err != nil {
				pm.logger.Warn("Failed to add paired device to discovery service", zap.Error(err))
			}
		}
	}
	
	return &response, nil
}

// IsPaired checks if a device is paired
func (pm *PairingManager) IsPaired(peerID string) bool {
	pm.devicesLock.RLock()
	defer pm.devicesLock.RUnlock()
	
	_, exists := pm.pairedDevices[peerID]
	return exists
}

// GetPairedDevices returns all paired devices
func (pm *PairingManager) GetPairedDevices() []PairedDevice {
	pm.devicesLock.RLock()
	defer pm.devicesLock.RUnlock()
	
	devices := make([]PairedDevice, 0, len(pm.pairedDevices))
	for _, device := range pm.pairedDevices {
		devices = append(devices, device)
	}
	return devices
}

// RemovePairedDevice removes a paired device
func (pm *PairingManager) RemovePairedDevice(peerID string) error {
	pm.devicesLock.Lock()
	defer pm.devicesLock.Unlock()
	
	if _, exists := pm.pairedDevices[peerID]; !exists {
		return fmt.Errorf("device not paired: %s", peerID)
	}
	
	delete(pm.pairedDevices, peerID)
	
	// Save paired devices
	if err := pm.savePairedDevices(); err != nil {
		pm.logger.Warn("Failed to save paired devices", zap.Error(err))
	}
	
	pm.logger.Info("Removed paired device", zap.String("peer_id", peerID))
	return nil
}

// UpdatePairedDevice updates a paired device's information
func (pm *PairingManager) UpdatePairedDevice(device PairedDevice) error {
	pm.devicesLock.Lock()
	defer pm.devicesLock.Unlock()
	
	if _, exists := pm.pairedDevices[device.PeerID]; !exists {
		return fmt.Errorf("device not paired: %s", device.PeerID)
	}
	
	pm.pairedDevices[device.PeerID] = device
	
	// Save paired devices
	if err := pm.savePairedDevices(); err != nil {
		pm.logger.Warn("Failed to save paired devices", zap.Error(err))
	}
	
	return nil
}

// handlePairingRequest handles an incoming pairing request
func (pm *PairingManager) handlePairingRequest(reader *bufio.Reader, writer *bufio.Writer, remotePeer peer.ID) {
	// Check if pairing is enabled
	if !pm.IsPairingEnabled() {
		sendPairingError(writer, "Pairing not enabled on this device")
		pm.logger.Warn("Received pairing request while pairing disabled", zap.String("peer_id", remotePeer.String()))
		return
	}
	
	// Read the request
	requestJSON, err := reader.ReadBytes('\n')
	if err != nil {
		pm.logger.Error("Failed to read pairing request", zap.Error(err))
		return
	}
	
	// Unmarshal the request
	var request PairingRequest
	if err := json.Unmarshal(requestJSON, &request); err != nil {
		sendPairingError(writer, "Invalid request format")
		pm.logger.Error("Failed to unmarshal pairing request", zap.Error(err))
		return
	}
	
	// Log the request
	pm.logger.Info("Received pairing request",
		zap.String("peer_id", remotePeer.String()),
		zap.String("device_name", request.DeviceName),
		zap.String("device_type", request.DeviceType))
	
	// Generate random data for verification code
	randomBytes := make([]byte, 16)
	if _, err := rand.Read(randomBytes); err != nil {
		sendPairingError(writer, "Internal error")
		pm.logger.Error("Failed to generate random data", zap.Error(err))
		return
	}
	randomData := base64.StdEncoding.EncodeToString(randomBytes)
	
	// Call the request handler
	accepted, err := pm.incomingHandler(request, remotePeer.String())
	if err != nil {
		sendPairingError(writer, fmt.Sprintf("Error: %v", err))
		pm.logger.Error("Error in pairing request handler", zap.Error(err))
		return
	}
	
	// Create response
	response := PairingResponse{
		Accepted:    accepted,
		DeviceName:  pm.config.DeviceName,
		DeviceType:  pm.config.DeviceType,
		Timestamp:   time.Now(),
		RandomData:  randomData,
		PeerID:      pm.host.ID().String(),
		Metadata:    map[string]string{},
		ValidUntil:  time.Time{}, // No expiration by default
	}
	
	// Add metadata
	response.Metadata["version"] = "1.0.0" // Example version
	
	// If rejected, add error message
	if !accepted {
		response.ErrorMessage = "Pairing request rejected"
	} else {
		// Generate verification code
		pairingCode := generateVerificationCode(request.RandomData, response.RandomData)
		
		// Add to paired devices if accepted
		device := PairedDevice{
			PeerID:     remotePeer.String(),
			DeviceName: request.DeviceName,
			DeviceType: request.DeviceType,
			LastSeen:   time.Now(),
			PairedAt:   time.Now(),
			Addresses:  []string{},
			Metadata:   request.Metadata,
		}
		
		// Get addresses
		for _, addr := range pm.host.Peerstore().Addrs(remotePeer) {
			device.Addresses = append(device.Addresses, addr.String())
		}
		
		// Add capabilities if present in metadata
		if caps, ok := request.Metadata["capabilities"]; ok {
			device.Capabilities = []string{caps}
		}
		
		// Add to paired devices
		pm.devicesLock.Lock()
		pm.pairedDevices[remotePeer.String()] = device
		pm.devicesLock.Unlock()
		
		// Save paired devices
		if err := pm.savePairedDevices(); err != nil {
			pm.logger.Warn("Failed to save paired devices", zap.Error(err))
		}
		
		pm.logger.Info("Accepted pairing request",
			zap.String("peer_id", remotePeer.String()),
			zap.String("verification_code", pairingCode))
	}
	
	// Marshal and send the response
	responseJSON, err := json.Marshal(response)
	if err != nil {
		pm.logger.Error("Failed to marshal pairing response", zap.Error(err))
		return
	}
	
	if _, err := writer.Write(responseJSON); err != nil {
		pm.logger.Error("Failed to write pairing response", zap.Error(err))
		return
	}
	
	if _, err := writer.WriteString("\n"); err != nil {
		pm.logger.Error("Failed to write newline", zap.Error(err))
		return
	}
	
	if err := writer.Flush(); err != nil {
		pm.logger.Error("Failed to flush writer", zap.Error(err))
		return
	}
}

// handleVerification handles verification of a pairing
func (pm *PairingManager) handleVerification(reader *bufio.Reader, writer *bufio.Writer, remotePeer peer.ID) {
	// This would implement the verification step if needed
	// For now, we'll use a simple verification code displayed on both devices
}

// Helper functions

// sendPairingError sends an error response
func sendPairingError(writer *bufio.Writer, message string) {
	// Create error response
	response := PairingResponse{
		Accepted:     false,
		ErrorMessage: message,
		Timestamp:    time.Now(),
	}
	
	// Marshal and send
	responseJSON, _ := json.Marshal(response)
	writer.Write(responseJSON)
	writer.WriteString("\n")
	writer.Flush()
}

// generateNonce generates a random nonce
func generateNonce() string {
	nonceBytes := make([]byte, 16)
	rand.Read(nonceBytes)
	return base64.URLEncoding.EncodeToString(nonceBytes)
}

// generateVerificationCode creates a human-readable verification code
func generateVerificationCode(initiatorRandom, responderRandom string) string {
	// Combine the random values
	combined := initiatorRandom + responderRandom
	
	// Create an HMAC-SHA256
	h := hmac.New(sha256.New, []byte(combined))
	h.Write([]byte(combined))
	digest := h.Sum(nil)
	
	// Use the first AuthCodeLength bytes to create a verification code
	// Format as a 6-digit code
	var code uint32
	for i := 0; i < AuthCodeLength; i++ {
		code = (code << 8) | uint32(digest[i])
	}
	
	// Limit to 6 digits
	code = code % 1000000
	
	// Format as a string with leading zeros
	return fmt.Sprintf("%06d", code)
}

// savePairedDevices saves paired devices to disk
func (pm *PairingManager) savePairedDevices() error {
	// In a real implementation, save to a file or database
	// For now, just log that we would save them
	pm.logger.Debug("Would save paired devices", zap.Int("count", len(pm.pairedDevices)))
	return nil
}

// loadPairedDevices loads paired devices from disk
func (pm *PairingManager) loadPairedDevices() error {
	// In a real implementation, load from a file or database
	// For now, just log that we would load them
	pm.logger.Debug("Would load paired devices")
	return nil
}

// getDiscoveryService gets the manual discovery service
func (pm *PairingManager) getDiscoveryService() (*ManualDiscovery, error) {
	node, ok := pm.host.(*Node)
	if !ok {
		return nil, errors.New("host is not a Node")
	}
	
	return node.discovery.GetManualDiscovery()
}

// CreatePairingDiscoveryService creates a discovery service for paired devices
type PairingDiscoveryService struct {
	manager       *PairingManager
	host          host.Host
	logger        *zap.Logger
	started       bool
	mutex         sync.RWMutex
}

// NewPairingDiscoveryService creates a new pairing discovery service
func NewPairingDiscoveryService(manager *PairingManager, host host.Host) *PairingDiscoveryService {
	return &PairingDiscoveryService{
		manager: manager,
		host:    host,
		logger:  manager.logger.With(zap.String("component", "pairing-discovery")),
		started: false,
	}
}

// Start starts the pairing discovery service
func (p *PairingDiscoveryService) Start() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	if p.started {
		return nil
	}
	
	// Connect to all paired devices
	for _, device := range p.manager.GetPairedDevices() {
		p.connectToPairedDevice(device)
	}
	
	p.started = true
	p.logger.Info("Started pairing discovery service")
	return nil
}

// Stop stops the pairing discovery service
func (p *PairingDiscoveryService) Stop() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	
	if !p.started {
		return nil
	}
	
	p.started = false
	p.logger.Info("Stopped pairing discovery service")
	return nil
}

// Name returns the name of the discovery service
func (p *PairingDiscoveryService) Name() string {
	return "paired"
}

// connectToPairedDevice connects to a paired device
func (p *PairingDiscoveryService) connectToPairedDevice(device PairedDevice) {
	// Skip if no addresses
	if len(device.Addresses) == 0 {
		p.logger.Warn("No addresses for paired device", zap.String("peer_id", device.PeerID))
		return
	}
	
	// Parse peer ID
	peerID, err := peer.Decode(device.PeerID)
	if err != nil {
		p.logger.Warn("Invalid peer ID for paired device", zap.String("peer_id", device.PeerID), zap.Error(err))
		return
	}
	
	// Create address info
	addrs := make([]multiaddr.Multiaddr, 0, len(device.Addresses))
	for _, addrStr := range device.Addresses {
		addr, err := multiaddr.NewMultiaddr(addrStr)
		if err != nil {
			p.logger.Warn("Invalid address for paired device",
				zap.String("peer_id", device.PeerID),
				zap.String("addr", addrStr),
				zap.Error(err))
			continue
		}
		addrs = append(addrs, addr)
	}
	
	// Add addresses to peerstore
	p.host.Peerstore().AddAddrs(peerID, addrs, time.Hour*24*7) // 1 week
	
	// Connect in a goroutine
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		
		// Try to connect
		if err := p.host.Connect(ctx, peer.AddrInfo{ID: peerID, Addrs: addrs}); err != nil {
			p.logger.Debug("Failed to connect to paired device",
				zap.String("peer_id", device.PeerID),
				zap.Error(err))
			return
		}
		
		p.logger.Info("Connected to paired device", zap.String("peer_id", device.PeerID))
		
		// Create peer info
		peerInfo := InternalPeerInfo{
			ID:           device.PeerID,
			Name:         device.DeviceName,
			Addrs:        device.Addresses,
			LastSeen:     time.Now(),
			Capabilities: make(map[string]string),
			DeviceType:   device.DeviceType,
		}
		
		// Add capabilities
		for _, cap := range device.Capabilities {
			peerInfo.Capabilities[cap] = "true"
		}
		
		// Update peer information
		device.LastSeen = time.Now()
		p.manager.UpdatePairedDevice(device)
	}()
} 