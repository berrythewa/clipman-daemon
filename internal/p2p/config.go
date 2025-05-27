// Package p2p provides clipboard synchronization using libp2p
package p2p

import (
	"fmt"
	"path/filepath"

	"github.com/berrythewa/clipman-daemon/internal/config"
	"github.com/berrythewa/clipman-daemon/internal/p2p/discovery"
	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	// "github.com/libp2p/go-libp2p/core/peer"
	noise "github.com/libp2p/go-libp2p/p2p/security/noise"
	"github.com/multiformats/go-multiaddr"
	"go.uber.org/zap"
	"strings"
)

// SyncConfig holds all sync-related configuration
type SyncConfig struct {
	// Core Sync Settings
	Enabled           bool     `json:"enable_sync"`
	SyncOverInternet  bool     `json:"sync_over_internet"`
	UseRelayNodes     bool     `json:"use_relay_nodes"`
	ListenPort        int      `json:"listen_port"`
	PeerIdentity      string   `json:"peer_identity"`
	DiscoveryMethod   string   `json:"discovery_method"` // "mdns", "dht", or "manual"
	DHTBootstrapPeers []string `json:"dht_bootstrap_peers"` // Bootstrap peers for DHT
	BootstrapPeers    []string `json:"bootstrap_peers"`    // Bootstrap peers for the network
	
	// Peer Persistence
	PersistDiscoveredPeers bool   `json:"persist_discovered_peers"` // Whether to save discovered peers to disk
	DiscoveredPeersPath    string `json:"discovered_peers_path"`    // Path to store discovered peers
	AutoReconnectToPeers   bool   `json:"auto_reconnect_to_peers"`  // Whether to auto-reconnect to known peers
	MaxStoredPeers         int    `json:"max_stored_peers"`         // Maximum number of peers to store
	
	// Clipboard Sync Options
	ClipboardTypes    []string `json:"clipboard_types"`  // "text", "image", "files"
	AutoCopyFromPeers bool     `json:"auto_copy_from_peers"`
	MaxClipboardSizeKB int     `json:"max_clipboard_size_kb"`
	ClipboardHistorySize int   `json:"clipboard_history_size"`
	ClipboardBlacklistApps []string `json:"clipboard_blacklist_apps"`
	
	// File Transfer Options
	EnableFileSharing bool     `json:"enable_file_sharing"`
	RequireFileConfirmation bool `json:"require_file_confirmation"`
	DefaultDownloadFolder string `json:"default_download_folder"`
	AutoAcceptFromPeers []string `json:"auto_accept_from_peers"`
	MaxFileSizeMB      int     `json:"max_file_size_mb"`
	
	// Privacy & Security
	AllowOnlyKnownPeers bool   `json:"allow_only_known_peers"`
	TrustedPeers       []string `json:"trusted_peers"`
	RequireApprovalPin bool    `json:"require_approval_pin"`
	LogPeerActivity    bool    `json:"log_peer_activity"`
	
	// Developer & Debug Options
	DebugLogging       bool    `json:"debug_logging"`
	ShowPeerDebugInfo  bool    `json:"show_peer_debug_info"`
	DisableMultiplexing bool   `json:"disable_multiplexing"`
	ForceDirectConnectionOnly bool `json:"force_direct_connection_only"`
	
	// DHT Discovery Options
	DHTPersistentStorage bool   `json:"dht_persistent_storage"` // Whether to store DHT data on disk
	DHTStoragePath       string `json:"dht_storage_path"`      // Path to store DHT data
	DHTServerMode        bool   `json:"dht_server_mode"`       // Whether to run DHT in server mode
	UseAsRelay          bool   `json:"use_as_relay"`          // Whether to act as a relay for other peers
	PersistDHTData      bool   `json:"persist_dht_data"`      // Whether to persist DHT data
	
	// Pairing Options
	PairingEnabled bool   `json:"pairing_enabled"`
	DeviceName     string `json:"device_name"`
	DeviceType     string `json:"device_type"`
	PairingTimeout int    `json:"pairing_timeout"`
}

// NodeConfig contains configuration specific to the libp2p node
type NodeConfig struct {
	ListenAddresses []string
	EnableNAT       bool
	EnableRelay     bool
	PeerIdentity    string
}

// DiscoveryConfig contains discovery-specific configuration
type DiscoveryConfig struct {
	Method          string
	EnableMDNS      bool
	EnableDHT       bool
	BootstrapPeers  []string
}

// ProtocolConfig contains protocol-specific configuration
type ProtocolConfig struct {
	EnablePubSub     bool
	SignMessages     bool
	StrictSigning    bool
}

// LoadSyncConfig extracts sync config from the global config
func LoadSyncConfig(cfg *config.Config) *SyncConfig {
	// Get paths for file storage
	paths := cfg.GetPaths()
	
	// Map from config.Config.Sync to our internal SyncConfig
	syncCfg := &SyncConfig{
		// Core Sync Settings
		Enabled:           cfg.Sync.Enabled,
		SyncOverInternet:  cfg.Sync.SyncOverInternet,
		UseRelayNodes:     cfg.Sync.UseRelayNodes,
		ListenPort:        cfg.Sync.ListenPort,
		DiscoveryMethod:   cfg.Sync.DiscoveryMethod,
		DHTBootstrapPeers: []string{}, // Default is empty, will use the hard-coded defaults
		
		// Peer Persistence
		PersistDiscoveredPeers: true, 
		DiscoveredPeersPath:    filepath.Join(paths.DataDir, "peers.json"),
		AutoReconnectToPeers:   true,
		MaxStoredPeers:         100, // Reasonable limit to prevent excessive storage
		
		// Clipboard Sync Options
		ClipboardTypes:         cfg.Sync.ClipboardTypes,
		AutoCopyFromPeers:      cfg.Sync.AutoCopyFromPeers,
		MaxClipboardSizeKB:     cfg.Sync.MaxClipboardSizeKB,
		ClipboardHistorySize:   cfg.Sync.ClipboardHistorySize,
		ClipboardBlacklistApps: cfg.Sync.ClipboardBlacklistApps,
		
		// File Transfer Options
		EnableFileSharing:       cfg.Sync.EnableFileSharing,
		RequireFileConfirmation: cfg.Sync.RequireFileConfirmation,
		DefaultDownloadFolder:   cfg.Sync.DefaultDownloadFolder,
		MaxFileSizeMB:           cfg.Sync.MaxFileSizeMB,
		AutoAcceptFromPeers:     []string{}, // Not exposed in user config yet
		
		// Privacy & Security
		AllowOnlyKnownPeers: cfg.Sync.AllowOnlyKnownPeers,
		TrustedPeers:        cfg.Sync.TrustedPeers,
		RequireApprovalPin:  cfg.Sync.RequireApprovalPin,
		LogPeerActivity:     cfg.Sync.LogPeerActivity,
		
		// Developer & Debug Options
		DebugLogging:              cfg.Sync.DebugLogging,
		ShowPeerDebugInfo:         cfg.Sync.ShowPeerDebugInfo,
		DisableMultiplexing:       cfg.Sync.DisableMultiplexing,
		ForceDirectConnectionOnly: cfg.Sync.ForceDirectConnectionOnly,
		
		// DHT Discovery Options
		DHTPersistentStorage: true, // Enable persistent storage by default
		DHTStoragePath:       filepath.Join(paths.DataDir, "dht"),
		DHTServerMode:        false, // Default to client mode for lower resource usage
		UseAsRelay:          false, // Default to not act as a relay
		PersistDHTData:      true,  // Default to persisting DHT data
		BootstrapPeers:      []string{}, // Default to empty bootstrap peers list
		
		// Pairing Options - use device info from main config
		PairingEnabled: true, // Enable pairing by default
		DeviceName:     cfg.DeviceName,
		DeviceType:     "desktop", // Hardcoded for now, should be determined based on platform
		PairingTimeout: 300, // 5 minutes default timeout
	}
	
	return syncCfg
}

// ValidateSyncConfig validates the sync configuration
func ValidateSyncConfig(cfg *SyncConfig) error {
	// Validate discovery method
	switch cfg.DiscoveryMethod {
	case "mdns", "dht", "manual", "paired":
		// Valid options
	default:
		return fmt.Errorf("invalid discovery method: %s", cfg.DiscoveryMethod)
	}
	
	// Validate clipboard types
	for _, t := range cfg.ClipboardTypes {
		switch t {
		case "text", "image", "files":
			// Valid options
		default:
			return fmt.Errorf("invalid clipboard type: %s", t)
		}
	}
	
	// Validate size limits
	if cfg.MaxClipboardSizeKB < 0 {
		return fmt.Errorf("max_clipboard_size_kb cannot be negative")
	}
	
	if cfg.MaxFileSizeMB < 0 {
		return fmt.Errorf("max_file_size_mb cannot be negative")
	}
	
	// Validate pairing timeout
	if cfg.PairingTimeout < 0 {
		return fmt.Errorf("pairing_timeout cannot be negative")
	}
	
	// Ensure device name is set when pairing is enabled
	if cfg.PairingEnabled && cfg.DeviceName == "" {
		return fmt.Errorf("device_name must be set when pairing is enabled")
	}
	
	// Ensure device type is set when pairing is enabled
	if cfg.PairingEnabled && cfg.DeviceType == "" {
		return fmt.Errorf("device_type must be set when pairing is enabled")
	}
	
	return nil
}

// GetNodeConfig extracts node-specific configuration
func GetNodeConfig(cfg *SyncConfig) NodeConfig {
	listenAddrs := []string{}
	
	// If a specific port is configured, use it
	if cfg.ListenPort > 0 {
		listenAddrs = append(listenAddrs, 
			fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", cfg.ListenPort),
			fmt.Sprintf("/ip4/0.0.0.0/udp/%d/quic", cfg.ListenPort),
		)
	} else {
		// Otherwise use dynamic ports
		listenAddrs = append(listenAddrs,
			"/ip4/0.0.0.0/tcp/0",
			"/ip4/0.0.0.0/udp/0/quic",
		)
	}
	
	return NodeConfig{
		ListenAddresses: listenAddrs,
		EnableNAT:       cfg.SyncOverInternet,
		EnableRelay:     cfg.UseRelayNodes,
		PeerIdentity:    cfg.PeerIdentity,
	}
}

// GetDiscoveryConfig extracts discovery-specific configuration
func GetDiscoveryConfig(cfg *SyncConfig) DiscoveryConfig {
	// Default bootstrap peers for the DHT
	defaultBootstrapPeers := []string{
		// IPFS bootstrap nodes (useful as general DHT bootstrap nodes)
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmNnooDu7bfjPFoTZYxMNLWUQJyrVwtbZg5gBMjTezGAJN",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmQCU2EcMqAqQPR2i9bChDtGNJchTbq5TbXJJ16u19uLTa",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmbLHAnMoJPWSCR5Zhtx6BHJX9KiKNN6tpvbUcqanj75Nb",
		"/dnsaddr/bootstrap.libp2p.io/p2p/QmcZf59bWwK5XFi76CZX8cbJ4BhTzzA3gU1ZjYZcYW3dwt",
		
		// These nodes would ideally be replaced with dedicated Clipman bootstrap nodes
		// in a production environment
	}
	
	// Use configured bootstrap peers if available, otherwise use defaults
	bootstrapPeers := defaultBootstrapPeers
	if len(cfg.DHTBootstrapPeers) > 0 {
		bootstrapPeers = cfg.DHTBootstrapPeers
	}
	
	return DiscoveryConfig{
		Method:          cfg.DiscoveryMethod,
		EnableMDNS:      cfg.DiscoveryMethod == "mdns" || cfg.DiscoveryMethod == "",
		EnableDHT:       cfg.DiscoveryMethod == "dht",
		BootstrapPeers:  bootstrapPeers,
	}
}

// GetProtocolConfig extracts protocol-specific configuration
func GetProtocolConfig(cfg *SyncConfig) ProtocolConfig {
	return ProtocolConfig{
		EnablePubSub:     true, // Default to enabled
		SignMessages:     true, // Default to signing messages
		StrictSigning:    false, // Default to not requiring signatures
	}
}

// MapToLibp2pOptions converts sync config to libp2p options
func MapToLibp2pOptions(cfg *SyncConfig, nodeCfg NodeConfig, logger *zap.Logger) ([]libp2p.Option, error) {
	options := []libp2p.Option{}
	
	// Set up peer identity
	if nodeCfg.PeerIdentity != "" {
		// Load identity from string
		privKey, err := loadIdentityFromString(nodeCfg.PeerIdentity)
		if err != nil {
			return nil, fmt.Errorf("failed to load peer identity: %w", err)
		}
		options = append(options, libp2p.Identity(privKey))
	}
	
	// Set up security
	options = append(options, libp2p.Security(noise.ID, noise.New))
	
	// Set up listen addresses
	for _, addr := range nodeCfg.ListenAddresses {
		// Validate address
		_, err := multiaddr.NewMultiaddr(addr)
		if err != nil {
			logger.Warn("Invalid listen address", zap.String("addr", addr), zap.Error(err))
			continue
		}
		options = append(options, libp2p.ListenAddrStrings(addr))
	}
	
	// Enable NAT traversal if internet sync is enabled
	if nodeCfg.EnableNAT {
		options = append(options, libp2p.NATPortMap())
	}
	
	// Enable relay if configured
	if nodeCfg.EnableRelay {
		options = append(options, libp2p.EnableRelay())
	}
	
	// Disable multiplexing if configured
	if cfg.DisableMultiplexing {
		// This would need to be implemented with the correct libp2p options
		logger.Warn("DisableMultiplexing is configured but not implemented")
	}
	
	return options, nil
}

// loadIdentityFromString loads a libp2p identity from a string
func loadIdentityFromString(identityStr string) (crypto.PrivKey, error) {
	// This is a placeholder - you would implement proper deserialization
	// of the private key from the string format used in your config
	return nil, fmt.Errorf("loading identity from string not implemented")
}

// SaveIdentityToString converts a private key to a storable string format
func SaveIdentityToString(privKey crypto.PrivKey) (string, error) {
	// This is a placeholder - you would implement proper serialization
	// of the private key to a secure string format for config storage
	return "", fmt.Errorf("saving identity to string not implemented")
}

// ClipboardTypesToContentTypes converts string type names to ContentType values
func ClipboardTypesToContentTypes(types []string) []string {
	result := make([]string, 0, len(types))
	for _, t := range types {
		switch strings.ToLower(t) {
		case "text":
			result = append(result, "text/plain")
		case "image":
			result = append(result, "image/png", "image/jpeg", "image/gif")
		case "files":
			result = append(result, "application/octet-stream")
		}
	}
	return result
}

// ExpandPath expands paths like ~ to absolute paths
func ExpandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := filepath.Abs(filepath.Join(filepath.Dir("~"), filepath.Base("~")))
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}

// GetDiscoveryConfig converts SyncConfig to discovery.Config
func ConvertToDiscoveryConfig(cfg *SyncConfig) *discovery.Config {
	return &discovery.Config{
		EnableMDNS:           cfg.DiscoveryMethod == "mdns" || cfg.DiscoveryMethod == "all",
		EnableDHT:            cfg.DiscoveryMethod == "dht" || cfg.DiscoveryMethod == "all" || cfg.SyncOverInternet,
		BootstrapPeers:       cfg.BootstrapPeers,
		PersistPeers:         cfg.PersistDiscoveredPeers,
		PeersPath:            cfg.DiscoveredPeersPath,
		MaxStoredPeers:       cfg.MaxStoredPeers,
		AutoReconnect:        cfg.AutoReconnectToPeers,
		DHTServerMode:        cfg.UseAsRelay,
		DHTPersistentStorage: cfg.PersistDHTData,
		DHTStoragePath:       cfg.DHTStoragePath,
	}
} 