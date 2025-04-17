// Package types defines common types used throughout the application
package types

import (
	"time"
)

// DiscoveryPeerInfo contains detailed information about a discovered peer
// This type is used for communication between sync and discovery packages
type DiscoveryPeerInfo struct {
	ID           string            // Peer ID (string representation of peer.ID)
	Name         string            // Human-readable name
	Addrs        []string          // Multiaddrs for connecting to the peer
	Groups       []string          // Groups the peer is a member of
	LastSeen     time.Time         // When the peer was last seen
	Capabilities map[string]string // Peer capabilities
	Version      string            // Software version
	DeviceType   string            // Type of device (desktop, mobile, etc.)
}

// ToBasicPeerInfo converts a DiscoveryPeerInfo to the simpler PeerInfo type
func (p DiscoveryPeerInfo) ToBasicPeerInfo() PeerInfo {
	return PeerInfo{
		ID:         p.ID,
		Name:       p.Name,
		DeviceType: p.DeviceType,
		LastSeen:   p.LastSeen,
	}
}

// FromBasicPeerInfo creates a DiscoveryPeerInfo from the simpler PeerInfo type
func FromBasicPeerInfo(p PeerInfo) DiscoveryPeerInfo {
	return DiscoveryPeerInfo{
		ID:           p.ID,
		Name:         p.Name,
		DeviceType:   p.DeviceType,
		LastSeen:     p.LastSeen,
		Groups:       []string{},
		Addrs:        []string{},
		Capabilities: make(map[string]string),
		Version:      "",
	}
} 