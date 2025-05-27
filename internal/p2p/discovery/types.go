// Package discovery provides peer discovery mechanisms for the sync package
package discovery

import (
	"github.com/berrythewa/clipman-daemon/internal/types"
	"github.com/libp2p/go-libp2p/core/peer"
	"time"
)

// Service represents a method for discovering peers
type Service interface {
	// Start starts the discovery service
	Start() error
	
	// Stop stops the discovery service
	Stop() error
	
	// Name returns the name of the discovery service
	Name() string
}

// Config represents configuration for discovery services
type Config struct {
	// Whether to use mDNS for local discovery
	EnableMDNS bool
	
	// Whether to use DHT for internet discovery
	EnableDHT bool
	
	// Bootstrap peers for DHT
	BootstrapPeers []string
	
	// Whether to persist discovered peers to disk
	PersistPeers bool
	
	// Path to store discovered peers
	PeersPath string
	
	// Maximum number of peers to store
	MaxStoredPeers int
	
	// Whether to automatically reconnect to known peers
	AutoReconnect bool
	
	// Whether to use DHT server mode (more resource intensive)
	DHTServerMode bool
	
	// Whether to store DHT data on disk
	DHTPersistentStorage bool
	
	// Path to store DHT data
	DHTStoragePath string
}

// AddrInfoToString converts a peer.AddrInfo to a single multiaddress string
func AddrInfoToString(info peer.AddrInfo) string {
	if len(info.Addrs) == 0 {
		return ""
	}
	
	// Take the first address for simplicity
	addr := info.Addrs[0]
	return addr.String() + "/p2p/" + info.ID.String()
}

// PeerAddrInfoToDiscoveryPeerInfo converts a peer.AddrInfo to types.DiscoveryPeerInfo
func PeerAddrInfoToDiscoveryPeerInfo(info peer.AddrInfo) types.DiscoveryPeerInfo {
	addrs := make([]string, 0, len(info.Addrs))
	for _, addr := range info.Addrs {
		addrs = append(addrs, addr.String())
	}
	
	return types.DiscoveryPeerInfo{
		ID:           info.ID.String(),
		Name:         "Peer-" + info.ID.ShortString(),
		Addrs:        addrs,
		LastSeen:     time.Now(),
		DeviceType:   "unknown",
		Version:      "",
		Groups:       []string{},
		Capabilities: make(map[string]string),
	}
}

// StringToPeerInfo converts a string to peer info
func StringToPeerInfo(addrStr string) (types.PeerInfo, error) {
	// This is a placeholder - would need to be implemented
	// to convert a multiaddress string to a peer info object
	return types.PeerInfo{}, nil
} 